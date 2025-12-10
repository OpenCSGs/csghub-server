package component

// CaptchaComponent defines the interface for captcha generation and verification
type CaptchaComponent interface {
	// Generate creates a new captcha and returns its ID, base64-encoded image, answer, and any error
	Generate() (string, string, string, error)

	// Verify checks if the provided answer matches the captcha with the given ID
	Verify(id, answer string) (bool, error)

	// VerifyWithUserIdentity checks if the provided answer matches the captcha with the given ID,
	// and associates the verification attempt with the user's identity (username or clientIP)
	// This helps with rate limiting and abuse prevention
	VerifyWithUserIdentity(id, answer, username, clientIP string) (string, bool, error)
}
