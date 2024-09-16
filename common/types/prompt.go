package types

type PromptReq struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	CurrentUser string `json:"current_user"`
	Path        string `json:"path"`
}

type Prompt struct {
	Title     string   `json:"title"`
	Content   string   `json:"content"`
	Language  string   `json:"language"`
	Tags      []string `json:"tags"`
	Type      string   `json:"type"` // "text|image|video|audio"
	Source    string   `json:"source"`
	Author    string   `json:"author"`
	Time      string   `json:"time"`
	Copyright string   `json:"copyright"`
	Feedback  []string `json:"feedback"`
	FilePath  string   `json:"file_path"`
}

type Conversation struct {
	Uuid        string   `json:"uuid" binding:"required"`
	Message     string   `json:"message" binding:"required"`
	Temperature *float64 `json:"temperature"`
}

type ConversationReq struct {
	Conversation
	CurrentUser string `json:"current_user"`
}

type ConversationTitle struct {
	Uuid  string `json:"uuid" binding:"required"`
	Title string `json:"title" binding:"required"`
}

type ConversationTitleReq struct {
	ConversationTitle
	CurrentUser string `json:"current_user"`
}

type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type LLMReqBody struct {
	Model       string       `json:"model"`
	Messages    []LLMMessage `json:"messages"`
	Stream      bool         `json:"stream"`
	Temperature float64      `json:"temperature"`
}

type ConversationMessageReq struct {
	Uuid        string `json:"uuid"`
	Id          int64  `json:"id"`
	CurrentUser string `json:"current_user"`
}

type LLMResponse struct {
	Id                string      `json:"id"`
	Object            string      `json:"object"`
	Created           int64       `json:"created"`
	Model             string      `json:"model"`
	SystemFingerprint string      `json:"system_fingerprint"`
	Choices           []LLMChoice `json:"choices"`
}

type LLMChoice struct {
	Index        int        `json:"index"`
	Delta        LLMMessage `json:"delta"`
	LogProbs     string     `json:"logprobs"`
	FinishReason string     `json:"finish_reason"`
}

type LLMDelta struct {
	Content string `json:"content"`
}
