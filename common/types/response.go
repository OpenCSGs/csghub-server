package types

type Response struct {
	Message string `json:"message"`
	Data    any    `json:"data"`
}

type ResponseWithTotal struct {
	Message string `json:"message"`
	Data    any    `json:"data"`
	Total   int    `json:"total"`
}

type APIInternalServerError struct{}

type APIBadRequest struct{}
