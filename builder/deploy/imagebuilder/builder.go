package imagebuilder

type Builder interface {
	Build(*BuildRequest) (*BuildResponse, error)
	Status(*StatusRequest) (*StatusResponse, error)
	Logs(*LogsRequest) (*LogsResponse, error)
}
