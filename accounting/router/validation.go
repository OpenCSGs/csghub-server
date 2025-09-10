package router

import (
	"time"

	"regexp"

	"github.com/go-playground/validator/v10"
)

var (
	phoneRegex = regexp.MustCompile(`^1[3-9]\d{9}$`)
)

func OptionalDateFormat(fl validator.FieldLevel) bool {
	dateStr := fl.Field().String()
	if len(dateStr) == 0 {
		return true
	}
	_, err := time.Parse("2006-01-02", dateStr)
	return err == nil
}

func OptionalYearMonthFormat(fl validator.FieldLevel) bool {
	dateStr := fl.Field().String()
	if len(dateStr) == 0 {
		return false
	}
	_, err := time.Parse("2006-01", dateStr)
	return err == nil
}

func OptionalPhoneFormat(fl validator.FieldLevel) bool {
	return phoneRegex.MatchString(fl.Field().String())
}
