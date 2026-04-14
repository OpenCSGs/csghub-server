package sensitive

import (
	"encoding/json"
	"regexp"
	"strings"
)

type RiskLevel string

const (
	RiskLevelSafe          RiskLevel = "Safe"
	RiskLevelUnsafe        RiskLevel = "Unsafe"
	RiskLevelControversial RiskLevel = "Controversial"
)
const (
	SafetyRegex = `Safety:\s*(Safe|Unsafe|Controversial)`
)

type LLMResponseParser interface {
	Parse(content string) *CheckResult
}

type ChainParser struct {
	parsers []LLMResponseParser
}

func NewChainParser(safetyRegex string) LLMResponseParser {
	return &ChainParser{
		parsers: []LLMResponseParser{
			&QwenGuardRegexParser{SafetyRegex: safetyRegex},
			&JSONParser{},
		},
	}
}

func (c *ChainParser) Parse(content string) *CheckResult {
	for _, parser := range c.parsers {
		res := parser.Parse(content)
		if res != nil {
			return res
		}
	}
	return &CheckResult{IsSensitive: false}
}

// QwenGuardRegexParser implements the parsing logic for Qwen3Guard model format
type QwenGuardRegexParser struct {
	SafetyRegex string
}

func (p *QwenGuardRegexParser) Parse(content string) *CheckResult {
	safetyRegex := p.SafetyRegex
	if safetyRegex == "" {
		safetyRegex = SafetyRegex
	}
	safePattern := regexp.MustCompile(safetyRegex)

	safeMatch := safePattern.FindStringSubmatch(content)
	if len(safeMatch) < 2 {
		return nil // Not matched, try next parser
	}

	label := safeMatch[1]

	// If it's safe, return early
	if label != string(RiskLevelUnsafe) {
		return &CheckResult{IsSensitive: false}
	}

	return &CheckResult{IsSensitive: true, Reason: content}
}

// JSONParser tries to parse standard JSON response
type JSONParser struct{}

func (p *JSONParser) Parse(content string) *CheckResult {
	content = strings.TrimSpace(content)

	// Remove markdown code block if present
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	var result LLMCheckResult
	err := json.Unmarshal([]byte(content), &result)
	if err == nil {
		if result.IsSensitive() {
			return &CheckResult{IsSensitive: true, Reason: content}
		}
		return &CheckResult{IsSensitive: false}
	}

	return nil // Not matched, try next parser
}

type LLMCheckResult struct {
	RiskLevel      string `json:"risk_level"`
	CategoryLabels string `json:"category_labels"`
}

func (p *LLMCheckResult) IsSensitive() bool {
	return p.RiskLevel == string(RiskLevelUnsafe)
}
