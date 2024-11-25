package checker

import (
	"bufio"
	"encoding/base64"
	"strings"

	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/common/config"
)

var contentChecker sensitive.SensitiveChecker
var localWordChecker *DFA

func Init(config *config.Config) {
	if !config.SensitiveCheck.Enable {
		panic("SensitiveCheck is not enable")
	}
	//init aliyun green checker
	contentChecker = sensitive.NewAliyunGreenCheckerFromConfig(config)
	//init local word checker
	localWordChecker = NewDFA()

	localWordChecker.BuildDFA(getSensitiveWordList(config.Moderation.EncodedSensitiveWords))
}

func getSensitiveWordList(encodedWords string) []string {
	r := base64.NewDecoder(base64.StdEncoding, strings.NewReader(encodedWords))
	s := bufio.NewScanner(r)
	s.Split(commaSplit)

	var words []string
	for s.Scan() {
		words = append(words, s.Text())
	}

	return words
}

// Custom split function that splits on commas
func commaSplit(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if len(data) == 0 {
		// When there's no data, return normally
		return 0, nil, nil
	}

	// Find the first comma
	if i := strings.IndexByte(string(data), ','); i >= 0 {
		// We've found a comma, return the part before it
		return i + 1, data[:i], nil
	}

	// If we've reached EOF and there's data left, return it
	if atEOF {
		return len(data), data, nil
	}

	// If we haven't found a comma and we're not at EOF,
	// we need more data
	return 0, nil, nil
}
