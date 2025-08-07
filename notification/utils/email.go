package utils

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func ExtractDisplayNameFromEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) < 2 {
		return email
	}

	caser := cases.Title(language.English)
	return caser.String(parts[0])
}
