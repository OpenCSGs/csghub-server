//go:build !saas

package email

func emailSubjectPrefix(string) string {
	return ""
}
