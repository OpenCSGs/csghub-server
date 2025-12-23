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

func NormalizePhoneArea(phoneArea string) string {
	if !strings.HasPrefix(phoneArea, "+") {
		return fmt.Sprintf("+%s", phoneArea)
	}
	return phoneArea
}
