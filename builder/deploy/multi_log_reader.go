package deploy

type MultiLogReader struct {
	buildLogs <-chan string
	runLogs   <-chan string
}

func NewMultiLogReader(logs ...<-chan string) *MultiLogReader {
	r := new(MultiLogReader)
	r.buildLogs = logs[0]
	r.runLogs = logs[1]
	return r
}

func (r *MultiLogReader) BuildLog() <-chan string {
	return r.buildLogs
}

func (r *MultiLogReader) RunLog() <-chan string {
	return r.runLogs
}
