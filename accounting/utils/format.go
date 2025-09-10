package utils

import (
	"errors"
	"time"
)

func ValidateDateTimeFormat(timeStr, layout string) bool {
	_, err := time.Parse(layout, timeStr)
	return err == nil
}

func ValidateQueryDate(startDateStr, endDateStr, layout string) (string, string, error) {
	if !ValidateDateTimeFormat(startDateStr, layout) || !ValidateDateTimeFormat(endDateStr, layout) {
		return "", "", errors.New("Bad request datetime format")
	}

	endDate, err := time.Parse(layout, endDateStr)
	if err != nil {
		return "", "", errors.New("Invalid end_time format")
	}

	endDate = endDate.Add(24 * time.Hour)

	startDate := startDateStr
	endDateStrModified := endDate.Format(layout)

	return startDate, endDateStrModified, nil
}
