package checker

import (
	"bufio"
	"context"
	"encoding/base64"
	"log/slog"
	"strings"
	"time"

	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/builder/store/database"
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

	//init default local word checker, to make sure it is not nil
	localWordChecker = NewDFA()
	localWordChecker.BuildDFA(getSensitiveWordList(config.Moderation.EncodedSensitiveWords))

	go refreshLocalWordChecker()
}

// InitWithContentChecker supports custom sensitive checker, this func mostly used in unit test
func InitWithContentChecker(config *config.Config, checker sensitive.SensitiveChecker) {
	if !config.SensitiveCheck.Enable {
		panic("SensitiveCheck is not enable")
	}

	if checker == nil {
		panic("param checker can not be nil")
	}
	contentChecker = checker
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

// refreshLocalWordChecker periodically refreshes the local sensitive word checker by
// loading sensitive words from the database and rebuilding the DFA. It runs in an
// infinite loop with a delay between iterations. In case of an error while retrieving
// the word list, it logs the error and retries after a delay.
func refreshLocalWordChecker() {
	const interval = 5 * time.Minute
	wordsetStore := database.NewSensitiveWordSetStore()
	wordsetFilter := &database.SensitiveWordSetFilter{}
	wordsetFilter.Enabled(true)
	for {
		//load sensitive words from database
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		wordSets, err := wordsetStore.List(ctx, wordsetFilter)
		cancel()
		if err != nil {
			slog.Error("refreshLocalWordChecker failed to load sensitive word list from db", slog.Any("error", err))
			time.Sleep(interval)
			continue
		}

		if len(wordSets) == 0 {
			slog.Info("refreshLocalWordChecker skip as sensitive word list is empty in db")
			time.Sleep(interval)
			continue
		}

		newChecker := NewDFA()
		var words []string
		for _, wordSet := range wordSets {
			words = append(words, strings.Split(wordSet.WordList, ",")...)
		}

		newChecker.BuildDFA(words)
		// update the reference
		localWordChecker = newChecker
		slog.Info("refreshLocalWordChecker success", slog.Int("word_count", len(words)), slog.Int("word_set_count", len(wordSets)))

		time.Sleep(interval)
	}
}
