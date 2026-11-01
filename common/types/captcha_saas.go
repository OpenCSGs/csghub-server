package types

// CaptchaVerifyRequest represents the request body for captcha verification
// @Description Request body for captcha verification
// @Accept json
// @Produce json
type CaptchaVerifyRequest struct {
	// ID is the captcha identifier
	ID string `json:"id" binding:"required"`
	// Answer is the user-provided captcha answer
	Answer string `json:"answer" binding:"required"`
}

// CaptchaVerifyResponse represents the response for captcha verification
// @Description Response for captcha verification
// @Accept json
// @Produce json
type CaptchaVerifyResponse struct {
	// Valid indicates whether the captcha was verified successfully
	Valid bool   `json:"valid"`
	Token string `json:"token"`
}

const (
	// CaptchaTokenPrefix is the prefix for verification tokens in Redis
	CaptchaTokenPrefix = "captcha:token:"
)
