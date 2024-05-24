package types

// Event is an event that can be emitted by a client
type Event struct {
	Module   string `json:"m" example:"space"`
	ID       string `json:"id" example:"space_card"`
	Value    string `json:"v" example:"1"`
	ClientID string `json:"c_id,omitempty" example:""`
	ClientIP string `json:"c_ip,omitempty" example:""`
	//reserved for future use
	Extension string `json:"ext,omitempty" example:""`
}
