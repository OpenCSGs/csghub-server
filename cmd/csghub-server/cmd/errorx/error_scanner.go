package errorx

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// ScanResult stores scanning results
type ScanResult struct {
	FilePath    string
	Line        int
	Column      int
	Code        string
	IssueType   string
	Description string
}

// Define error detection constants
const (
	IssueTypeErrorsNew   = "errors.New()"
	IssueTypeFmtErrorf   = "fmt.Errorf()"
	IssueTypeErrorsWrap  = "errors.Wrap()"
	IssueTypeErrorsWrapf = "errors.Wrapf()"
)

var (
	scanDir     string
	scanVerbose bool
)

// errorScanCmd represents the error scanner command
var errorScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan Go files for error usage issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		return scanErrors(scanDir, scanVerbose)
	},
}

func init() {
	errorScanCmd.Flags().StringVarP(&scanDir, "dir", "d", ".", "Directory path to scan")
	errorScanCmd.Flags().BoolVarP(&scanVerbose, "verbose", "v", false, "Show detailed scanning process")
}

// Keywords to detect meaningful error messages
var errorKeywords = []string{
	"error", "fail", "invalid", "unexpected", "missing",
	"cannot", "unable", "invalid", "malformed", "corrupted",
}

// Format placeholders for fmt.Errorf detection
var formatPlaceholders = []string{"%w", "%v", "%s", "%d", "%f"}

// scanFile scans a single file using AST to detect potential error usage issues
func scanFile(filePath string) []ScanResult {
	results := []ScanResult{}

	// Create file set for tracking line numbers
	fset := token.NewFileSet()

	// Parse file into AST
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		fmt.Printf("Failed to parse file %s: %v\n", filePath, err)
		return results
	}

	// Create visitor to traverse AST
	visitor := &errorVisitor{
		fset:    fset,
		file:    node,
		results: &results,
		path:    filePath,
	}

	// Traverse AST
	ast.Walk(visitor, node)

	return results
}

// errorVisitor implements ast.Visitor interface to traverse AST and detect error creation functions

type errorVisitor struct {
	fset    *token.FileSet
	file    *ast.File
	results *[]ScanResult
	path    string
}

// Visit implements ast.Visitor interface
func (v *errorVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}

	// Check function call expressions
	if callExpr, ok := node.(*ast.CallExpr); ok {
		v.checkCallExpr(callExpr)
	}

	return v
}

// checkCallExpr checks if a function call expression is an error creation function
func (v *errorVisitor) checkCallExpr(callExpr *ast.CallExpr) {
	// Get function selector expression
	selectorExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	// Get function package and name
	pkgIdent, ok := selectorExpr.X.(*ast.Ident)
	if !ok {
		return
	}

	funcName := selectorExpr.Sel.Name
	pkgName := pkgIdent.Name

	// Get position information
	pos := v.fset.Position(callExpr.Pos())

	// Get complete code line
	codeLine := v.getCodeLine(pos.Line)

	// Check errors.New()
	if pkgName == "errors" && funcName == "New" && len(callExpr.Args) == 1 {
		v.checkErrorsNew(callExpr, pos, codeLine)
	}

	// Check fmt.Errorf()
	if pkgName == "fmt" && funcName == "Errorf" && len(callExpr.Args) >= 1 {
		v.checkFmtErrorf(callExpr, pos, codeLine)
	}

	// Check errors.Wrap() and errors.Wrapf()
	if pkgName == "errors" && (funcName == "Wrap" || funcName == "Wrapf") {
		v.checkErrorsWrap(callExpr, pos, codeLine, funcName)
	}
}

// checkErrorsNew checks errors.New() calls
func (v *errorVisitor) checkErrorsNew(callExpr *ast.CallExpr, pos token.Position, codeLine string) {
	// Ensure first argument is a string literal
	if basicLit, ok := callExpr.Args[0].(*ast.BasicLit); ok && basicLit.Kind == token.STRING {
		// Extract string content (remove quotes)
		message := strings.Trim(basicLit.Value, `"'`)

		// Check if error message contains keywords
		if !containsErrorKeyword(message) {
			*v.results = append(*v.results, ScanResult{
				FilePath:    v.path,
				Line:        pos.Line,
				Column:      pos.Column,
				Code:        codeLine,
				IssueType:   IssueTypeErrorsNew,
				Description: "errors.New() call lacks meaningful error message",
			})
		}
	}
}

// checkFmtErrorf checks fmt.Errorf() calls
func (v *errorVisitor) checkFmtErrorf(callExpr *ast.CallExpr, pos token.Position, codeLine string) {
	// Ensure first argument is a string literal
	if basicLit, ok := callExpr.Args[0].(*ast.BasicLit); ok && basicLit.Kind == token.STRING {
		// Extract string content (remove quotes)
		format := strings.Trim(basicLit.Value, `"'`)

		// Check if it contains %w or other format placeholders
		if !containsFormatPlaceholder(format) {
			*v.results = append(*v.results, ScanResult{
				FilePath:    v.path,
				Line:        pos.Line,
				Column:      pos.Column,
				Code:        codeLine,
				IssueType:   IssueTypeFmtErrorf,
				Description: "fmt.Errorf() call lacks format placeholders or error wrapping",
			})
		}
	}
}

// checkErrorsWrap checks errors.Wrap() or errors.Wrapf() calls
func (v *errorVisitor) checkErrorsWrap(callExpr *ast.CallExpr, pos token.Position, codeLine string, funcName string) {
	issueType := IssueTypeErrorsWrap
	if funcName == "Wrapf" {
		issueType = IssueTypeErrorsWrapf
	}

	// Check if parameter count is correct
	expectedArgs := 2
	if funcName == "Wrapf" {
		expectedArgs = 3
	}

	if len(callExpr.Args) < expectedArgs {
		*v.results = append(*v.results, ScanResult{
			FilePath:    v.path,
			Line:        pos.Line,
			Column:      pos.Column,
			Code:        codeLine,
			IssueType:   issueType,
			Description: fmt.Sprintf("%s call has insufficient arguments", issueType),
		})
	}
}

// getCodeLine retrieves the complete code line based on line number
func (v *errorVisitor) getCodeLine(lineNum int) string {
	// Read file content directly to get accurate code line
	content, err := os.ReadFile(v.path)
	if err != nil {
		return fmt.Sprintf("Line %d (failed to read file content)", lineNum)
	}

	// Split file content by lines
	lines := strings.Split(string(content), "\n")

	// Ensure line number is valid
	if lineNum > 0 && lineNum <= len(lines) {
		// Line numbers start from 1, but slice indices start from 0
		return strings.TrimSpace(lines[lineNum-1])
	}

	// Return placeholder if line number is invalid
	return fmt.Sprintf("Line %d", lineNum)
}

// containsErrorKeyword checks if string contains error keywords
func containsErrorKeyword(s string) bool {
	for _, keyword := range errorKeywords {
		if strings.Contains(strings.ToLower(s), strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

// containsFormatPlaceholder checks if string contains format placeholders
func containsFormatPlaceholder(s string) bool {
	for _, placeholder := range formatPlaceholders {
		if strings.Contains(s, placeholder) {
			return true
		}
	}
	return false
}

// printResults outputs scanning results
func printResults(results []ScanResult, verbose bool) {
	if len(results) == 0 {
		slog.Info("No error usage issues found")
		return
	}

	// Group results by file path
	resultsByFile := make(map[string][]ScanResult)
	for _, result := range results {
		resultsByFile[result.FilePath] = append(resultsByFile[result.FilePath], result)
	}

	// Output results statistics
	totalIssues := len(results)
	slog.Info("Scan results", slog.Int("total_issues", totalIssues))

	// Output detailed information by file
	for filePath, fileResults := range resultsByFile {
		fmt.Printf("File: %s\n", filePath)
		for _, result := range fileResults {
			fmt.Printf("  line %d, column %d: %s\n", result.Line, result.Column, result.Code)
			if verbose {
				fmt.Printf("  issue type: %s\n", result.IssueType)
				fmt.Printf("  description: %s\n", result.Description)
				fmt.Printf("  suggestion: %s\n", getSuggestion(result.IssueType))
			}
			fmt.Println()
		}
	}

	// Output summary information
	slog.Info("Issue type summary")
	typeCount := make(map[string]int)
	for _, result := range results {
		typeCount[result.IssueType]++
	}
	for issueType, count := range typeCount {
		slog.Info("Issue type count", slog.String("type", issueType), slog.Int("count", count))
	}
}

// getSuggestion provides suggestions based on issue type
func getSuggestion(issueType string) string {
	switch issueType {
	case IssueTypeErrorsNew:
		return "Use or add custom error struct in errorx package to provide more context, e.g., 'errorx.ErrForbidden'"
	case IssueTypeFmtErrorf:
		return "Use %w to wrap original errors or %v to add variable information, e.g., 'failed to open file: %w'"
	case IssueTypeErrorsWrap:
		return "Ensure error object and description are provided, e.g., 'errors.Wrap(err, \"failed to process\")'"
	case IssueTypeErrorsWrapf:
		return "Ensure error object, format string and parameters are provided, e.g., 'errors.Wrapf(err, \"failed to process %s\", id)'"
	default:
		return "Please check error handling best practices"
	}
}

// List of directories to skip
var skipDirs = []string{".git", "vendor", "_mocks", "node_modules", "dist"}

// shouldSkipDir checks if a directory should be skipped
func shouldSkipDir(dir string) bool {
	dirName := filepath.Base(dir)
	for _, skip := range skipDirs {
		if dirName == skip {
			return true
		}
	}
	return false
}

// shouldSkipFile checks if a file should be skipped
func shouldSkipFile(filePath string) bool {
	// Skip non-Go files
	if filepath.Ext(filePath) != ".go" {
		return true
	}

	// Skip test files
	if strings.HasSuffix(filePath, "_test.go") {
		return true
	}

	return false
}

// scanErrors scans the specified directory for error usage issues
func scanErrors(dir string, verbose bool) error {
	slog.Info("Starting directory scan", slog.String("directory", dir))
	slog.Info("Skipping non-Go files and _test.go files")

	// Initialize results slice
	var results []ScanResult
	resultsChan := make(chan []ScanResult, 100)
	doneChan := make(chan bool)
	fileCount := 0

	// Start a goroutine to collect results
	go func() {
		for fileResults := range resultsChan {
			results = append(results, fileResults...)
			fileCount++
		}
		doneChan <- true
	}()

	// Walk through directory
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			if shouldSkipDir(path) {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip files that don't need processing
		if shouldSkipFile(path) {
			return err
		}

		// Scan file content
		fileResults := scanFile(path)
		resultsChan <- fileResults

		return nil
	})

	if err != nil {
		slog.Error("Error during directory walk", slog.Any("error", err))
		return fmt.Errorf("error during scanning: %w", err)
	}

	// Close channel and wait for collection to complete
	close(resultsChan)
	<-doneChan

	// Print detailed results
	printResults(results, verbose)

	slog.Info("Scan completed", slog.Int("file_count", fileCount))
	return nil
}
