package common

import (
	"fmt"
	"strings"
)

// UniqueSpaceAppName generates a unique app name for space deployment
func UniqueSpaceAppName(prefix, namespace, name string, spaceID int64) string {
	encodedSpaceID := numberToString(spaceID)
	// max length of uniqueAppName is 63
	avaiMaxLen := 63 - len(encodedSpaceID) - 3
	nameSpaceAndName := fmt.Sprintf("%s-%s", namespace, name)
	if len(nameSpaceAndName) > avaiMaxLen {
		nameSpaceAndName = nameSpaceAndName[:avaiMaxLen]
	}
	uniqueAppName := fmt.Sprintf("%s-%s-%s", prefix, nameSpaceAndName, encodedSpaceID)
	return strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(uniqueAppName, "_", "-"), ".", "-"))
}

func parseUniqueSpaceAppName(spaceAppName string) (spaceID int64, err error) {
	nameParts := strings.Split(spaceAppName, "-")
	spaceIDStr := nameParts[len(nameParts)-1]
	// decode space id
	return stringToNumber(spaceIDStr)
}

// NumberToString encodes a number into a shorter string representation without padding
func numberToString(num int64) string {
	alphabet := "0123456789abcdefghijklmnopqrstuvwxyz"
	var encodedBuilder strings.Builder
	base := int64(len(alphabet))

	for num > 0 {
		remainder := num % base
		num /= base
		encodedBuilder.WriteByte(alphabet[remainder])
	}

	// Reverse the encoded string since we've built it backwards
	encodedStr := encodedBuilder.String()
	runes := []rune(encodedStr)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}

	return string(runes)
}

// StringToNumber decodes a string back into the original number without padding
func stringToNumber(encoded string) (int64, error) {
	alphabet := "0123456789abcdefghijklmnopqrstuvwxyz"
	alphabetMap := make(map[rune]int64)
	for i, c := range alphabet {
		alphabetMap[c] = int64(i)
	}

	var num int64
	base := int64(len(alphabet))
	encodedRunes := []rune(encoded)

	for _, r := range encodedRunes {
		num = num*base + alphabetMap[r]
	}

	return num, nil
}
