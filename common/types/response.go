package types

type Response struct {
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

type ResponseWithTotal struct {
	Msg   string `json:"msg"`
	Data  any    `json:"data"`
	Total int    `json:"total"`
}

type APIInternalServerError struct{}

type APIBadRequest struct{}

type APIUnauthorized struct{}
