package sensitive

import (
	"context"
	"log/slog"
	"strings"

	"opencsg.com/csghub-server/builder/sensitive/internal"
	"opencsg.com/csghub-server/builder/sensitive/internal/ahocorasick"
	"opencsg.com/csghub-server/common/types"
)

// TextModerationResponseData refer to aliyun green text moderation response item
type TextModerationResponseData struct {
	// Labels.
	//
	// example:
	//
	// porn
	Labels *string `json:"labels,omitempty"`
	// The JSON string used to locate the cause.
	//
	// example:
	//
	// {\\"detectedLanguage\\":\\"ar\\",\\"riskTips\\":\\"sexuality_Suggestive\\",\\"riskWords\\":\\"pxxxxy\\",\\"translatedContent\\":\\"pxxxxy sxxxx\\"}
	Reason *string `json:"reason,omitempty"`
}

// ACAutomation represents an immutable Aho-Corasick automaton for sensitive word detection
// It is thread-safe and cannot be modified after initialization
type ACAutomation struct {
	matcher *ahocorasick.Matcher
	tagMap  map[int]string // Pattern index -> tag
	words   []string       // index -> word
}

// NewACAutomation creates a new immutable AC automaton with initial data
// It returns a new instance every time it's called
func NewACAutomation(data *internal.SensitiveWordData) SensitiveChecker {
	return &ACAutomation{
		tagMap:  data.TagMap,
		words:   data.Words,
		matcher: ahocorasick.NewStringMatcher(data.Words),
	}
}

// PassTextCheck implements the SensitiveChecker interface for ImmutableAC
func (iac *ACAutomation) PassTextCheck(ctx context.Context, scenario types.SensitiveScenario, text string) (*CheckResult, error) {
	detectResult := iac.detect(text)
	if detectResult != nil {
		slog.InfoContext(ctx, "ACAutomation PassTextCheck detected sensitive word",
			slog.String("reason", *detectResult.Reason))
		return &CheckResult{
			IsSensitive: true,
			Reason:      *detectResult.Reason,
		}, nil
	}
	return &CheckResult{
		IsSensitive: false,
	}, nil
}

// PassImageCheck implements the SensitiveChecker interface for ImmutableAC
func (iac *ACAutomation) PassImageCheck(ctx context.Context, scenario types.SensitiveScenario, ossBucketName, ossObjectName string) (*CheckResult, error) {
	slog.WarnContext(ctx, "PassImageCheck not implemented in Immutable AC checker")
	return &CheckResult{
		IsSensitive: false,
	}, nil
}

// PassImageURLCheck implements the SensitiveChecker interface for ImmutableAC
func (iac *ACAutomation) PassImageURLCheck(ctx context.Context, scenario types.SensitiveScenario, imageURL string) (*CheckResult, error) {
	slog.WarnContext(ctx, "PassImageURLCheck not implemented in Immutable AC checker")
	return &CheckResult{
		IsSensitive: false,
	}, nil
}

// PassLLMCheck implements the SensitiveChecker interface for ImmutableAC
func (iac *ACAutomation) PassLLMCheck(ctx context.Context, scenario types.SensitiveScenario, text string, sessionId string, accountId string) (*CheckResult, error) {
	if scenario != types.ScenarioLLMQueryModeration && scenario != types.ScenarioLLMResModeration {
		slog.WarnContext(ctx, "PassLLMCheck received unsupported scenario", slog.String("scenario", string(scenario)))
		return &CheckResult{
			IsSensitive: false,
		}, nil
	}
	detectResult := iac.detect(text)
	if detectResult != nil {
		slog.InfoContext(ctx, "ACAutomation PassLLMCheck detected sensitive word",
			slog.String("reason", *detectResult.Reason))
		return &CheckResult{
			IsSensitive: true,
			Reason:      *detectResult.Reason,
		}, nil
	}
	return &CheckResult{
		IsSensitive: false,
	}, nil
}

// detect implements the detection logic for ImmutableAC
func (iac *ACAutomation) detect(text string) *TextModerationResponseData {
	t := cleanText(strings.ToLower(text))

	hits := iac.matcher.MatchThreadSafeWithPos([]byte(t))

	seen := make(map[string]struct{})
	for _, hit := range hits {
		if hit.Hint < 0 || hit.Hint >= len(iac.words) {
			continue
		}
		word := iac.words[hit.Hint]
		tag := iac.tagMap[hit.Hint]
		key := tag + "|" + word

		contextStart := hit.StartPos - 10
		if contextStart < 0 {
			contextStart = 0
		}
		contextEnd := hit.EndPos + 10
		if contextEnd > len(t) {
			contextEnd = len(t)
		}

		var reasonBuilder strings.Builder
		reasonBuilder.WriteString(key)
		reasonBuilder.WriteString("|[text: ")
		if contextStart > 0 {
			reasonBuilder.WriteString("...")
		}
		reasonBuilder.WriteString(t[contextStart:contextEnd])
		if contextEnd < len(t) {
			reasonBuilder.WriteString("...")
		}
		reasonBuilder.WriteString("]")

		fullReason := reasonBuilder.String()

		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		return &TextModerationResponseData{
			Labels: &tag,
			Reason: &fullReason,
		}
	}
	return nil
}

func cleanText(text string) string {
	replacer := strings.NewReplacer(
		" ", "", "\t", "", "\n", "", "*", "", "@", "", "-", "", ".", "", ",", "", "_", "",
	)
	t := replacer.Replace(text)

	runes := []rune(t)
	for i, r := range runes {
		if r >= 65281 && r <= 65374 {
			runes[i] = r - 65248
		}
		if r == 12288 {
			runes[i] = ' '
		}
	}
	return string(runes)
}
