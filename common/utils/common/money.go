package common

import "math"

// ConvertCentsToYuan converts cents to yuan, keeping up to 8 decimal.
func ConvertCentsToYuan(cents float64) float64 {
	yuan := cents / 100.0
	scale := math.Pow10(8) // 10^8 = 100000000
	return math.Floor(yuan*scale) / scale
}
