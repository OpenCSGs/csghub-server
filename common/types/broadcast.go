package types

// content: free for user to input
// bc_type: 'banner', 'message'
// theme: 'light', 'dark'
// status: 'active', 'inactive'
type Broadcast struct {
	ID      int64  `json:"id"`
	Content string `json:"content"`
	BcType  string `json:"bc_type"`
	Theme   string `json:"theme"`
	Status  string `json:"status"`
}
