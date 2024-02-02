package inference

type App interface {
	Predict(string) (string, error)
}
