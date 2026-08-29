package utils

import (
	"errors"
	"time"

	"opencsg.com/csghub-server/common/config"
)

// timeLayoutWithOffset carries an explicit offset so timestamptz comparisons
// are unambiguous regardless of the database session timezone.
const timeLayoutWithOffset = "2006-01-02 15:04:05-07:00"

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

// ConvertToGlobalTimeZone parses value as global-timezone local time and
// returns the equivalent timestamp string with an explicit offset for the DB.
func ConvertToGlobalTimeZone(value, layout string) (string, error) {
	timeLoc := config.GetGlobalTimeZone()
	t, err := time.ParseInLocation(layout, value, timeLoc)
	if err != nil {
		return "", err
	}
	return t.Format(timeLayoutWithOffset), nil
}

// ConvertDateRangeToGlobalTimeZone parses an inclusive [start, end] date range
// in the global timezone and returns an explicit-offset [start, end+1day) range.
func ConvertDateRangeToGlobalTimeZone(start, end, layout string) (string, string, error) {
	timeLoc := config.GetGlobalTimeZone()
	s, err := time.ParseInLocation(layout, start, timeLoc)
	if err != nil {
		return "", "", err
	}
	e, err := time.ParseInLocation(layout, end, timeLoc)
	if err != nil {
		return "", "", err
	}
	e = time.Date(e.Year(), e.Month(), e.Day()+1, 0, 0, 0, 0, e.Location())
	return s.Format(timeLayoutWithOffset), e.Format(timeLayoutWithOffset), nil
}
