package sensitive

type TextScanResponseDataItemResult struct {
	Scene string `json:"scene"`
	Label string `json:"label"`
	//0~100
	Rate float32 `json:"rate"`
	//pass,review,block
	Suggestion string `json:"suggestion"`
}
type TextScanResponseDataItem struct {
	Code    int                              `json:"code"`
	Msg     string                           `json:"msg"`
	Content string                           `json:"content"`
	Results []TextScanResponseDataItemResult `json:"results"`
	TaskId  string                           `json:"taskId"`
}

type TextScanResponse struct {
	Code      int                        `json:"code"`
	Msg       string                     `json:"msg"`
	Data      []TextScanResponseDataItem `json:"data"`
	RequestID string                     `json:"requestId"`
}
