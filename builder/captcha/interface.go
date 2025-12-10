package captcha

type Interface interface {
	Generate() (string, string, string, error)
	Verify(id, answer string) (bool, error)
}
