package imagerunner

type Runner interface {
	Run(*RunRequest) (*RunResponse, error)
	Status(*StatusRequest) (*StatusResponse, error)
	Logs(*LogsRequest) (*LogsResponse, error)
}
