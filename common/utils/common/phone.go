package common

import (
	"fmt"
	"strings"

	"github.com/nyaruka/phonenumbers"
)

func GetCountryCodeByPhoneArea(phone string, phoneArea string) (string, error) {
	phoneNumber, err := phonenumbers.Parse(fmt.Sprintf("%s%s", phoneArea, phone), "")
	if err != nil {
		return "", err
	}
	countryCode := phonenumbers.GetRegionCodeForNumber(phoneNumber)
	if countryCode == "" {
		return "", fmt.Errorf("country code is empty for phone area:%s", phoneArea)
	}
	return countryCode, nil
}

func GetPhoneAreaByCountryCode(phone string, countryCode string) (string, error) {
	num, err := phonenumbers.Parse(phone, countryCode)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("+%d", num.GetCountryCode()), nil
}

func IsValidNumber(phone string, phoneArea string) (bool, error) {
	num, err := phonenumbers.Parse(fmt.Sprintf("%s%s", phoneArea, phone), "")
	if err != nil {
		return false, err
	}
	return phonenumbers.IsValidNumber(num), nil
}

// NormalizePhoneArea canonicalizes a phone area to an E.164 dialing prefix
// such as "+86". The input may be a dialing prefix ("86", "+86") or an ISO
// 3166-1 alpha-2 country code ("CN", "+cn"): casdoor submits ISO codes in
// its countryCode form fields, while risk phone prefix rules are keyed by
// dialing prefixes, so alpha input is translated via the phonenumbers
// library. Unknown alpha codes keep the historical plus-prefix form and can
// never match the numeric rules.
func NormalizePhoneArea(phoneArea string) string {
	trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(phoneArea), "+"))
	if trimmed == "" {
		return phoneArea
	}
	if isAllDigits(trimmed) {
		return "+" + trimmed
	}
	if dial := phonenumbers.GetCountryCodeForRegion(strings.ToUpper(trimmed)); dial > 0 {
		return fmt.Sprintf("+%d", dial)
	}
	return "+" + trimmed
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}
