package parquet

import (
	"context"
	"errors"
	"io"
	"slices"

	"github.com/go-pay/errgroup"
	"github.com/minio/minio-go/v7"
	"github.com/parquet-go/parquet-go"
	"go.opentelemetry.io/otel/trace"
)

const (
	fileReadConcurrency = 32
)

type ParquetGoReader struct {
	ioClient IOClient
	tracer   trace.Tracer
	bucket   string
}

type groupReader struct {
	reader *parquet.GenericReader[row]
	limit  int64
	offset int64
}

// used by NewGenericRowGroupReader, which require type explicitly
type row any

// pack parquet.File and closer together, so we can close the file after read the content
type pfile struct {
	rc io.Closer
	pf *parquet.File
}

type loader struct {
	groupReaders []groupReader
	files        []io.Closer
	total        int64
	columns      []string
	columnTypes  []string
}

func (l *loader) close() {
	for _, f := range l.files {
		_ = f.Close()
	}
}

func NewParquetGoReader(ioClient IOClient, tracer trace.Tracer, bucket string) *ParquetGoReader {
	return &ParquetGoReader{
		ioClient: ioClient,
		tracer:   tracer,
		bucket:   bucket,
	}
}

func (p *ParquetGoReader) build(ctx context.Context, files []*pfile, limit, offset int64) []groupReader {
	_, span := p.tracer.Start(ctx, "calculate required groups")
	defer span.End()

	var done bool
	// recalculate required groups
	readers := []groupReader{}
	var processed int64
	for _, f := range files {
		if done {
			break
		}

		for _, rg := range f.pf.RowGroups() {
			if done {
				break
			}

			// the global start/end row index of current group
			start := processed
			end := processed + rg.NumRows()

			if offset >= start && offset <= end {

				// the local offset/limit size of current group
				var groupOffset = offset - processed
				var groupLimit = groupOffset + limit

				// groupLimit > nr: need more data from next group
				if nr := rg.NumRows(); groupLimit > nr {
					groupLimit = nr
					// the global offset should be end of current group
					offset = end
					// the global limit should be remaining rows need to be fetched
					limit -= (nr - groupOffset)
				} else {
					done = true
				}
				readers = append(readers, groupReader{
					reader: parquet.NewGenericRowGroupReader[row](rg),
					limit:  groupLimit,
					offset: groupOffset,
				})

			}
			processed += rg.NumRows()
		}

	}
	return readers
}

func (p *ParquetGoReader) loadFiles(ctx context.Context, paths []string) ([]*pfile, error) {
	var base int
	files := make([]*pfile, len(paths))
	for c := range slices.Chunk(paths, fileReadConcurrency) {
		g := errgroup.WithContext(ctx)
		for i := 0; i < len(c); i++ {
			index := base + i
			g.Go(func(ctx context.Context) error {
				pf, err := p.parquetFile(ctx, paths[index])
				if err != nil {
					return err
				}
				files[index] = pf
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return nil, err
		}
		base += len(c)
	}
	return files, nil
}

func (p *ParquetGoReader) loader(ctx context.Context, paths []string, limit, offset int64) (*loader, error) {
	files, err := p.loadFiles(ctx, paths)
	if err != nil {
		return nil, err
	}

	var total int64
	rc := []io.Closer{}
	columns := []string{}
	columnTypes := []string{}
	for _, schema := range files[0].pf.Metadata().Schema {
		if schema.Type == nil {
			continue
		}
		columns = append(columns, schema.Name)
		columnTypes = append(columnTypes, schema.Type.String())
	}

	for _, f := range files {
		total += f.pf.NumRows()
		rc = append(rc, f.rc)
	}

	return &loader{
		groupReaders: p.build(ctx, files, limit, offset),
		total:        total,
		files:        rc,
		columns:      columns,
		columnTypes:  columnTypes,
	}, nil

}

func (p *ParquetGoReader) parquetFile(ctx context.Context, path string) (*pfile, error) {
	var span trace.Span
	ctx, span = p.tracer.Start(ctx, "read parquet file")
	defer span.End()

	// we need to load file content later, so can't defer close here
	// instead, only close the object manually when there is error
	obj, err := p.ioClient.GetFileObject(ctx, p.bucket, path, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}

	pf, err := parquet.OpenFile(obj.reader, obj.size)
	if err != nil {
		_ = obj.reader.Close()
		return nil, err
	}

	return &pfile{
		rc: obj.reader,
		pf: pf,
	}, nil
}

func (p *ParquetGoReader) readRowsFromGroup(ctx context.Context, reader groupReader) ([]parquet.Row, error) {
	_, span := p.tracer.Start(ctx, "read rows")
	defer span.End()

	// ReadRows(buffer) method of the parquet reader reads up to len(buffer) rows into the buffer.
	// However, it does not guarantee that exactly len(buffer) rows will be read, so we must use the
	// returned number of rows (n) to check if we've reached the limit.
	rows := make([]parquet.Row, 0, reader.limit)
	tmp := make([]parquet.Row, 300)
	count := 0
	for {
		n, err := reader.reader.ReadRows(tmp)
		if n > 0 {
			rows = append(rows, tmp[:n]...)
			count += n
		}
		// break if we reach limit, or reach end of file
		if count > int(reader.limit) {
			rows = rows[:reader.limit]
			break
		}
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, err
		}
	}

	return rows, nil

}

func (p *ParquetGoReader) RowsWithCount(ctx context.Context, paths []string, limit, offset int64) ([]string, []string, [][]any, int64, error) {

	if len(paths) == 0 {
		return []string{}, []string{}, [][]any{}, 0, nil
	}

	loader, err := p.loader(ctx, paths, limit, offset)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	defer loader.close()

	data := [][]any{}
	for _, reader := range loader.groupReaders {

		rows, err := p.readRowsFromGroup(ctx, reader)
		if err != nil {
			return nil, nil, nil, 0, err
		}

		for i, row := range rows {
			if i < int(reader.offset) {
				continue
			}
			rr := []any{}
			for _, cell := range row {
				rr = append(rr, cell.String())
			}
			data = append(data, rr)
		}
	}
	return loader.columns, loader.columnTypes, data, loader.total, nil
}
