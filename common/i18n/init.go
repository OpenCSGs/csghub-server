package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

var LocalizerMap map[string]*i18n.Localizer
var Matcher language.Matcher

//go:embed *
var i18nConfigFiles embed.FS

func InitLocalizersFromEmbedFile() {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	LocalizerMap = make(map[string]*i18n.Localizer)

	fileList, err := i18nConfigFiles.ReadDir(".")
	if err != nil {
		slog.Error("Failed to read i18n config files", slog.Any("error", err))
		return
	}
	supportedLang := make([]language.Tag, 0)
	for _, file := range fileList {
		if !file.IsDir() {
			continue
		}
		lang := file.Name()

		mergedData, err := readAndMergeLanguageJsonFiles(lang)
		if err != nil {
			slog.Error("Failed to read and merge language files", slog.String("lang", lang), slog.Any("error", err))
			continue
		}
		bundle.MustParseMessageFileBytes(mergedData, lang+".json")
		localizer := i18n.NewLocalizer(bundle, lang)
		LocalizerMap[lang] = localizer
		tag, err := language.Parse(lang)
		if err != nil {
			slog.Error("Failed to parse language tag", slog.String("lang", lang), slog.Any("error", err))
			continue
		}
		supportedLang = append(supportedLang, tag)
	}
	Matcher = language.NewMatcher(supportedLang)
}

// readAndMergeLanguageFiles
// reads all JSON files in the specified language directory and merges them into a single JSON data
func readAndMergeLanguageJsonFiles(lang string) ([]byte, error) {
	langFiles, err := i18nConfigFiles.ReadDir(lang)
	if err != nil {
		return nil, fmt.Errorf("failed to read language directory %s: %w", lang, err)
	}

	mergedMap := make(map[string]interface{})

	for _, file := range langFiles {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(lang, file.Name())
		data, err := i18nConfigFiles.ReadFile(filePath)
		if err != nil {
			slog.Error("Failed to read i18n file", slog.String("file", filePath), slog.Any("error", err))
			continue
		}

		var fileMap map[string]interface{}
		if err := json.Unmarshal(data, &fileMap); err != nil {
			slog.Error("Failed to unmarshal JSON file", slog.String("file", filePath), slog.Any("error", err))
			continue
		}

		// merge to file map
		for key, value := range fileMap {
			mergedMap[key] = value
		}
	}

	// Convert merged map back to JSON format
	return json.Marshal(mergedMap)
}
