package types

// AIGatewayModel is the public model contract needed by agent provisioning.
type AIGatewayModel struct {
	ID                  string         `json:"id"`
	Task                string         `json:"task"`
	Metadata            map[string]any `json:"metadata"`
	SupportFunctionCall bool           `json:"support_function_call"`
}

type AIGatewayModelList struct {
	Data []AIGatewayModel `json:"data"`
}
