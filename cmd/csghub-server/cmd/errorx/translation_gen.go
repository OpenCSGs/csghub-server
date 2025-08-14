package errorx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var docGenCmd = &cobra.Command{
	Use:   "doc-gen",
	Short: "Generate documentation for error codes",
	Run: func(cmd *cobra.Command, args []string) {
		lh := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: false,
			Level:     slog.LevelInfo,
		})
		l := slog.New(lh)
		slog.SetDefault(l)

		docGen()
	},
}

const (
	errorxSourceGlob    = "common/errorx/error_*.go"
	i18nBaseDir         = "common/i18n"
	descriptionPrefix   = "Description"
	descriptionZHPrefix = "Description_ZH"

	outputMarkdownPath   = "docs/error_codes_en.md"
	outputMarkdownZHPath = "docs/error_codes_zh.md"
	// templatePath       = "docgen/template.md"
)

var langCodeRegex = regexp.MustCompile(`^[a-z]+(?:-[A-Z]+)?$`)

type ErrorInfo struct {
	Code           int
	ConstName      string
	Description    string // Description from comments
	Description_ZH string
	FullCode       string
	Translations   map[string]string // language code to translation
}

// DocData for error code documentation
type DocData struct {
	Title  string
	Errors []ErrorInfo
}

type MarkdownConfig struct {
	OutputPath    string
	Title         string
	IntroText     string
	ChapterFormat string
	DetailLabels  map[string]string // e.g., "Full Code", "Constant Name", "Description"
	Lang          string            // "en" or "zh"
}

func docGen() {
	_, currentFilePath, _, ok := runtime.Caller(0)
	if !ok {
		slog.Error("Failed to get current path")
	}
	currentDir := filepath.Dir(currentFilePath)
	projectRoot := filepath.Join(currentDir, "..", "..", "..", "..")
	dataDirPath := filepath.Clean(filepath.Join(projectRoot, errorxSourceGlob))
	// search for Go source files matching the pattern
	goFiles, err := filepath.Glob(dataDirPath)
	if err != nil {
		slog.Error("Failed to glob for source files", slog.Any("error", err))
	}
	if len(goFiles) == 0 {
		slog.Error("No error source files found matching pattern", slog.Any("source", errorxSourceGlob))
	}

	infosByFile := make(map[string][]ErrorInfo)
	allErrorFullCodes := make(map[string]string)

	for _, goFile := range goFiles {
		log.Printf("Processing file: %s", goFile)
		infosFromFile, err := parseGoSource(goFile)
		if err != nil {
			slog.Error("Failed to parse Go source file",
				slog.String("file", goFile),
				slog.Any("error", err))
		}

		var sortedInfosFromFile []ErrorInfo
		for _, info := range infosFromFile {
			if existingFile, exists := allErrorFullCodes[info.FullCode]; exists {
				slog.Error(fmt.Sprintf(
					"Duplicate full error code '%s' found. It exists in both '%s' and '%s'.",
					info.FullCode, existingFile, filepath.Base(goFile)))
				return
			}
			allErrorFullCodes[info.FullCode] = filepath.Base(goFile)
			sortedInfosFromFile = append(sortedInfosFromFile, *info)
		}
		sort.Slice(sortedInfosFromFile, func(i, j int) bool {
			return sortedInfosFromFile[i].Code < sortedInfosFromFile[j].Code
		})

		baseName := filepath.Base(goFile)
		infosByFile[baseName] = sortedInfosFromFile
	}
	outputDir := filepath.Join(projectRoot, i18nBaseDir)
	err = generateTranslationJSONs(infosByFile, outputDir)
	if err != nil {
		slog.Error("generate i18n json error", slog.Any("error", err))
	}

	// 1. Configure and generate English documentation
	enConfig := MarkdownConfig{
		OutputPath:    filepath.Join(projectRoot, outputMarkdownPath),
		Title:         "# Error Code Documentation",
		IntroText:     "This document lists all the custom error codes defined in the project, categorized by module.",
		ChapterFormat: "## %s Errors",
		DetailLabels: map[string]string{
			"FullCode":     "Error Code",
			"ConstantName": "Error Name",
			"Description":  "Description",
		},
		Lang: "en",
	}
	err = generateMarkdownDoc(infosByFile, enConfig)
	if err != nil {
		slog.Error("Failed to generate English markdown documentation", slog.Any("error", err))
	}

	// 2. Configure and generate Chinese documentation
	zhConfig := MarkdownConfig{
		OutputPath:    filepath.Join(projectRoot, outputMarkdownZHPath),
		Title:         "# 错误码文档",
		IntroText:     "本文档列出了项目中定义的所有自定义错误码，按模块分类。",
		ChapterFormat: "## %s 错误",
		DetailLabels: map[string]string{
			"FullCode":     "错误代码",
			"ConstantName": "错误名",
			"Description":  "描述",
		},
		Lang: "zh",
	}
	err = generateMarkdownDoc(infosByFile, zhConfig)
	if err != nil {
		slog.Error("Failed to generate Chinese markdown documentation", slog.Any("error", err))
	}
}

// parseGoSource parses a Go source file to extract error codes and their descriptions.
func parseGoSource(filePath string) (map[int]*ErrorInfo, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	infos := make(map[int]*ErrorInfo)
	var errPrefix string
	currentCode := -1

	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.ValueSpec:
			// capture `const err...Prefix = "..."`
			if len(x.Names) > 0 && strings.HasSuffix(x.Names[0].Name, "Prefix") {
				if basicLit, ok := x.Values[0].(*ast.BasicLit); ok {
					errPrefix = strings.Trim(basicLit.Value, `"`)
				}
			}

		case *ast.GenDecl:
			if x.Tok == token.CONST {
				for _, spec := range x.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok && !strings.HasSuffix(vs.Names[0].Name, "Prefix") {
						currentCode++
						info := getOrCreate(infos, currentCode)
						info.ConstName = vs.Names[0].Name
					}
				}
			}

			if x.Tok == token.VAR {
				for _, spec := range x.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						description, description_zh, translation := extractComment(vs.Doc)

						if cl, ok := vs.Values[0].(*ast.CompositeLit); ok {
							for _, elt := range cl.Elts {
								if kve, ok := elt.(*ast.KeyValueExpr); ok {
									if key, ok := kve.Key.(*ast.Ident); ok && key.Name == "code" {
										if val, ok := kve.Value.(*ast.Ident); ok {
											for _, existingInfo := range infos {
												if existingInfo.ConstName == val.Name {
													existingInfo.Description = description
													existingInfo.Description_ZH = description_zh
													existingInfo.Translations = translation
													break
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
		return true
	})

	if errPrefix == "" {
		return nil, fmt.Errorf("could not find err...Prefix constant in %s", filePath)
	}

	for code := range infos {
		infos[code].FullCode = fmt.Sprintf("%s-%d", errPrefix, code)
	}

	return infos, nil
}

func extractComment(doc *ast.CommentGroup) (string, string, map[string]string) {
	if doc == nil {
		return "", "", nil
	}
	var description, description_zh = "", ""
	var translations = make(map[string]string)
	for _, commentLine := range doc.List {
		line := strings.TrimSpace(strings.TrimPrefix(commentLine.Text, "//"))

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if langCodeRegex.MatchString(key) {
				translations[key] = value
			} else if key == descriptionPrefix {
				description = value
			} else if key == descriptionZHPrefix {
				description_zh = value
			}
		}

	}
	return description, description_zh, translations
}

func getOrCreate(m map[int]*ErrorInfo, code int) *ErrorInfo {
	if _, ok := m[code]; !ok {
		m[code] = &ErrorInfo{Code: code}
	}
	return m[code]
}

type TranslationEntry struct {
	Other string `json:"other"`
}

func generateTranslationJSONs(infosByFile map[string][]ErrorInfo, outputDir string) error {
	for goFileName, infos := range infosByFile {
		// 1. for current file
		// map[langCode]map[jsonKey]TranslationEntry
		translationsByLang := make(map[string]map[string]TranslationEntry)

		for _, info := range infos {
			jsonKey := fmt.Sprintf("error.%s", info.FullCode)
			for langCode, translationText := range info.Translations {
				if _, ok := translationsByLang[langCode]; !ok {
					translationsByLang[langCode] = make(map[string]TranslationEntry)
				}
				translationsByLang[langCode][jsonKey] = TranslationEntry{Other: translationText}
			}
		}

		jsonFileName := strings.TrimSuffix("err_"+strings.TrimPrefix(goFileName, "error_"), ".go") + ".json"

		// 2. for every file, generate json file
		for langCode, translationMap := range translationsByLang {
			// e.g., "common/i18n/en-US"
			langDir := filepath.Join(outputDir, langCode)
			if err := os.MkdirAll(langDir, 0755); err != nil {
				return fmt.Errorf("failed to create language directory %s: %w", langDir, err)
			}

			//  e.g., "common/i18n/en-US/error_auth.json"
			filePath := filepath.Join(langDir, jsonFileName)

			jsonData, err := json.MarshalIndent(translationMap, "", "    ")
			if err != nil {
				log.Printf("WARN: Failed to marshal JSON for %s in language %s: %v", goFileName, langCode, err)
				continue
			}

			if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
				log.Printf("WARN: Failed to write translation file %s: %v", filePath, err)
				continue
			}

			log.Printf("Successfully generated translation file: %s", filePath)
		}
	}
	return nil
}

func generateMarkdownDoc(infosByFile map[string][]ErrorInfo, config MarkdownConfig) error {
	var builder bytes.Buffer

	builder.WriteString(config.Title + "\n\n")
	builder.WriteString(config.IntroText + "\n\n")

	var sortedFiles []string
	for filename := range infosByFile {
		sortedFiles = append(sortedFiles, filename)
	}
	sort.Strings(sortedFiles)

	for _, goFileName := range sortedFiles {
		infos := infosByFile[goFileName]
		if len(infos) == 0 {
			continue
		}

		moduleName := strings.TrimSuffix(strings.TrimPrefix(goFileName, "error_"), ".go")
		chapterTitle := strings.ToUpper(string(moduleName[0])) + moduleName[1:]
		builder.WriteString(fmt.Sprintf(config.ChapterFormat+"\n\n", chapterTitle))

		for i, info := range infos {
			builder.WriteString(fmt.Sprintf("### `%s`\n\n", info.FullCode))

			builder.WriteString(fmt.Sprintf("- **%s:** `%s`\n", config.DetailLabels["FullCode"], info.FullCode))
			builder.WriteString(fmt.Sprintf("- **%s:** `%s`\n", config.DetailLabels["ConstantName"], info.ConstName))

			var description string
			if config.Lang == "zh" {
				description = info.Description_ZH
				// Fallback to English description if Chinese one is not provided
				if description == "" {
					description = info.Description
				}
			} else {
				description = info.Description
			}

			// Sanitize the description
			description = strings.ReplaceAll(description, "`", "\\`")
			builder.WriteString(fmt.Sprintf("- **%s:** %s\n", config.DetailLabels["Description"], description))

			if i < len(infos)-1 {
				builder.WriteString("\n---\n\n")
			} else {
				builder.WriteString("\n")
			}
		}
	}

	if err := os.WriteFile(config.OutputPath, builder.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write markdown documentation to %s: %w", config.OutputPath, err)
	}

	slog.Info("Successfully generated markdown documentation", "path", config.OutputPath)
	return nil
}
