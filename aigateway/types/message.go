package types

// refer to https://github.com/openai/openai-go/blob/main/chatcompletion.go#L1219
// refer to https://github.com/go-deepseek/deepseek/blob/main/response/response.go
type Message struct {
	Role             string `json:"role,omitempty"`
	Name             string `json:"name,omitempty"`
	Content          any    `json:"content,omitempty"`
	ReasoningContent string `json:"reasoning_content"`
}

func (m Message) IsStringContent() bool {
	_, ok := m.Content.(string)
	return ok
}

func (m Message) StringContent() string {
	content, ok := m.Content.(string)
	if ok {
		return content
	}
	contentList, ok := m.Content.([]any)
	if ok {
		var contentStr string
		for _, contentItem := range contentList {
			contentMap, ok := contentItem.(map[string]any)
			if !ok {
				continue
			}
			if contentMap["type"] == "text" {
				if subStr, ok := contentMap["text"].(string); ok {
					contentStr += subStr
				}
			}
		}
		return contentStr
	}
	return ""
}

func (m Message) ParseContent() []MessageContent {
	var contentList []MessageContent
	content, ok := m.Content.(string)
	if ok {
		contentList = append(contentList, MessageContent{
			Type: "text",
			Text: content,
		})
		return contentList
	}
	anyList, ok := m.Content.([]any)
	if ok {
		for _, contentItem := range anyList {
			contentMap, ok := contentItem.(map[string]any)
			if !ok {
				continue
			}
			switch contentMap["type"] {
			case "text":
				if subStr, ok := contentMap["text"].(string); ok {
					contentList = append(contentList, MessageContent{
						Type: "text",
						Text: subStr,
					})
				}
			case "image_url":
				if subObj, ok := contentMap["image_url"].(map[string]any); ok {
					contentList = append(contentList, MessageContent{
						Type: "image_url",
						ImageURL: &ImageURL{
							Url: subObj["url"].(string),
						},
					})
				}
			}
		}
		return contentList
	}
	return nil
}

type ImageURL struct {
	Url    string `json:"url,omitempty"`
	Detail string `json:"detail,omitempty"`
}

type MessageContent struct {
	Type     string    `json:"type,omitempty"`
	Text     string    `json:"text"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}
