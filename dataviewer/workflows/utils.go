package workflows

import (
	"bufio"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"go.temporal.io/sdk/activity"
	"opencsg.com/csghub-server/common/types"
	dvCom "opencsg.com/csghub-server/dataviewer/common"
)

func TransferFileObject(file *dvCom.RepoFile, subsetName, splitName string) dvCom.FileObject {
	return dvCom.FileObject{
		RepoFile:        file.Path,
		Size:            file.Size,
		LastCommit:      file.LastCommitSHA,
		Lfs:             file.Lfs,
		LfsRelativePath: file.LfsRelativePath,
		SubsetName:      subsetName,
		SplitName:       splitName,
		DownloadSize:    file.DownloadSize,
	}
}

func GetPatternFileList(path interface{}) []string {
	if path == nil {
		return []string{}
	}
	var (
		files []string
	)
	if slice, ok := path.([]interface{}); ok {
		for _, v := range slice {
			files = append(files, v.(string))
		}
	} else if slice, ok := path.([]string); ok {
		files = slice
	} else {
		files = []string{path.(string)}
	}
	return files
}

func ConvertRealFiles(splitFiles []string, filePaths []string, targetFiles map[string]*dvCom.RepoFile, subsetName, splitName string) []dvCom.FileObject {
	var phyFiles []dvCom.FileObject
	for _, filePattern := range splitFiles {
		if !strings.Contains(filePattern, dvCom.WILDCARD) || !doublestar.ValidatePathPattern(filePattern) {
			file, exists := targetFiles[filePattern]
			if exists {
				phyFiles = append(phyFiles, TransferFileObject(file, subsetName, splitName))
			}
			continue
		}

		for _, path := range filePaths {
			match, err := doublestar.PathMatch(filePattern, path)
			if err != nil {
				slog.Error("file pattern match", "error", err)
			}
			if match {
				file, exists := targetFiles[path]
				if exists {
					phyFiles = append(phyFiles, TransferFileObject(file, subsetName, splitName))
				}
			}
		}
	}
	slog.Debug("convert real files", slog.Any("splitFiles", splitFiles), slog.Any("phyFiles", phyFiles))
	return phyFiles
}

func GetCardDataMD5(finalCardData dvCom.CardData) string {
	hasher := md5.New()
	for _, info := range finalCardData.DatasetInfos {
		for _, split := range info.Splits {
			for _, file := range split.Origins {
				hasher.Write([]byte(fmt.Sprintf("%s-%s", file.RepoFile, file.LastCommit)))
			}
		}
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func GenerateWorkFlowID(req types.UpdateViewerReq) string {
	id := "dv-" + req.Namespace + "/" + req.Name + "-" + time.Now().Format("20060102.150405.000000000")
	return id
}

func GetCacheRepoPath(ctx context.Context, cacheDir string, req types.UpdateViewerReq) string {
	wfCtx := activity.GetInfo(ctx)
	workflowID := wfCtx.WorkflowExecution.ID
	segments := strings.Split(workflowID, "-")
	timeSeq := segments[len(segments)-1]
	return fmt.Sprintf("%s/%s/%s/%s/%s", strings.TrimSuffix(cacheDir, "/"), req.RepoType, req.Namespace, req.Name, timeSeq)
}

func CopyJsonArray(writeFile *os.File, reader io.ReadCloser, limitSize int64) (int64, error) {
	decoder := json.NewDecoder(reader)
	token, err := decoder.Token()
	if err != nil || token != json.Delim('[') {
		return 0, fmt.Errorf("invalid json array object error: %w", err)
	}
	writer := NewFileWriter(writeFile)
	encoder := json.NewEncoder(writer)
	_, err = writer.Write([]byte{'['})
	if err != nil {
		return 0, fmt.Errorf("write json begin delimiter error: %w", err)
	}
	count := 0
	var copyedBytes int64
	for decoder.More() {
		if copyedBytes >= limitSize {
			break
		}
		var obj map[string]interface{}
		if err := decoder.Decode(&obj); err != nil {
			return 0, fmt.Errorf("parse json object error: %w", err)
		}
		if count > 0 {
			_, err = writer.Write([]byte{','})
			if err != nil {
				return 0, fmt.Errorf("write comma error: %w", err)
			}
		}

		err = encoder.Encode(obj)
		if err != nil {
			return 0, fmt.Errorf("encode json object error: %w", err)
		}
		count++
		copyedBytes += int64(writer.GetWriteBytes())
	}
	_, err = writer.Write([]byte{']'})
	if err != nil {
		return 0, fmt.Errorf("write json end delimiter error: %w", err)
	}
	return copyedBytes, nil
}

func CopyFileContext(writeFile *os.File, reader io.ReadCloser, limitSize int64) (int64, error) {
	var copyedBytes int64
	scanner := bufio.NewScanner(reader)
	writer := bufio.NewWriter(writeFile)

	for scanner.Scan() {
		if copyedBytes >= limitSize {
			break
		}
		line := scanner.Text()
		n, err := writer.WriteString(fmt.Sprintln(line))
		if err != nil {
			return copyedBytes, fmt.Errorf("write content by line error: %w", err)
		}
		err = writer.Flush()
		if err != nil {
			return 0, fmt.Errorf("flush data error: %w", err)
		}
		copyedBytes += int64(n)
	}

	return copyedBytes, nil
}

func GetThreadNum(totalDataSize int64, maxThreadNum int) int {
	threadNum := int(totalDataSize / DataSizePerThread)
	if threadNum < 1 {
		threadNum = 1
	} else if threadNum > maxThreadNum {
		threadNum = maxThreadNum
	}
	return threadNum
}
