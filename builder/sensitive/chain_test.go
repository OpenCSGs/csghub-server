package sensitive_test

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	mockgreen "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/common/types/sensitive"
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	ss_type "opencsg.com/csghub-server/common/types/sensitive"
)

func TestChainImpl_AliYun_PassTextCheck(t *testing.T) {
	checker := mockgreen.NewMockSensitiveChecker(t)
	chain := sensitive.NewChainCheckerWithCheckers(checker)

	ctx := context.Background()
	scenario := types.ScenarioCommentDetection
	text := "test text"
	checker.EXPECT().PassTextCheck(ctx, scenario, text).Return(&ss_type.CheckResult{IsSensitive: false}, nil)

	result, err := chain.PassTextCheck(ctx, scenario, text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatalf("expected non-nil result, got nil")
	}
	if result.IsSensitive {
		t.Fatalf("expected non-sensitive result, got sensitive")
	}
}

func TestChainImpl_AliYun_PassTextCheck_Sensitive(t *testing.T) {
	checker := mockgreen.NewMockSensitiveChecker(t)
	chain := sensitive.NewChainCheckerWithCheckers(checker)

	ctx := context.Background()
	scenario := types.ScenarioCommentDetection
	text := "sensitive text"
	reason := "sensitive content"
	labels := "politics"
	requestId := "test-request-id"
	expectedReason := fmt.Sprintf("label:%s,reason:%s,requestId:%s", labels, reason, requestId)
	checker.EXPECT().PassTextCheck(ctx, scenario, text).Return(&ss_type.CheckResult{
		IsSensitive: true,
		Reason:      expectedReason,
	}, nil)

	result, err := chain.PassTextCheck(ctx, scenario, text)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatalf("expected non-nil result, got nil")
	}
	if !result.IsSensitive {
		t.Fatalf("expected sensitive result, got non-sensitive")
	}
	if result.Reason != expectedReason {
		t.Fatalf("expected reason %s, got %s", expectedReason, result.Reason)
	}
}

func TestChainImpl_AliYun_PassImageCheck(t *testing.T) {
	checker := mockgreen.NewMockSensitiveChecker(t)
	chain := sensitive.NewChainCheckerWithCheckers(checker)

	ctx := context.Background()
	scenario := types.ScenarioImageBaseLineCheck
	ossBucketName := "test-bucket"
	ossObjectName := "test-image.jpg"
	checker.EXPECT().PassImageCheck(ctx, scenario, ossBucketName, ossObjectName).
		Return(&ss_type.CheckResult{IsSensitive: false}, nil)

	result, err := chain.PassImageCheck(ctx, scenario, ossBucketName, ossObjectName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatalf("expected non-nil result, got nil")
	}
	if result.IsSensitive {
		t.Fatalf("expected non-sensitive result, got sensitive")
	}
}

func TestChainImpl_AliYun_PassImageCheck_Sensitive(t *testing.T) {
	checker := mockgreen.NewMockSensitiveChecker(t)
	chain := sensitive.NewChainCheckerWithCheckers(checker)

	ctx := context.Background()
	scenario := types.ScenarioImageBaseLineCheck
	ossBucketName := "test-bucket"
	ossObjectName := "test-image.jpg"
	checker.EXPECT().PassImageCheck(ctx, scenario, ossBucketName, ossObjectName).
		Return(&ss_type.CheckResult{IsSensitive: true, Reason: "politics"}, nil)

	result, err := chain.PassImageCheck(ctx, scenario, ossBucketName, ossObjectName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatalf("expected non-nil result, got nil")
	}
	if !result.IsSensitive {
		t.Fatalf("expected sensitive result, got non-sensitive")
	}
	if result.Reason != "politics" {
		t.Fatalf("expected reason %s, got %s", "politics", result.Reason)
	}
}

func TestChainImpl_AliYun_PassImageURLCheck(t *testing.T) {
	checker := mockgreen.NewMockSensitiveChecker(t)
	chain := sensitive.NewChainCheckerWithCheckers(checker)

	ctx := context.Background()
	scenario := types.ScenarioImageBaseLineCheck
	imageURL := "https://example.com/normal-image.jpg"

	checker.EXPECT().PassImageURLCheck(ctx, scenario, imageURL).Return(&ss_type.CheckResult{
		IsSensitive: false,
	}, nil)

	result, err := chain.PassImageURLCheck(ctx, scenario, imageURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatalf("expected non-nil result, got nil")
	}
	if result.IsSensitive {
		t.Fatalf("expected non-sensitive result, got sensitive")
	}
}

func TestChainImpl_AliYun_PassImageURLCheck_Sensitive(t *testing.T) {
	checker := mockgreen.NewMockSensitiveChecker(t)
	chain := sensitive.NewChainCheckerWithCheckers(checker)

	ctx := context.Background()
	scenario := types.ScenarioImageBaseLineCheck
	imageURL := "https://example.com/sensitive-image.jpg"
	labels := "politics"
	confidence := 95.0
	requestId := "test-request-id"

	checker.EXPECT().PassImageURLCheck(ctx, scenario, imageURL).Return(&ss_type.CheckResult{
		IsSensitive: true,
		Reason:      fmt.Sprintf("label:%s,confidence:%f,requestId:%s", labels, confidence, requestId),
	}, nil)
	result, err := chain.PassImageURLCheck(ctx, scenario, imageURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatalf("expected non-nil result, got nil")
	}
	if !result.IsSensitive {
		t.Fatalf("expected sensitive result, got non-sensitive")
	}
	expectedReason := fmt.Sprintf("label:%s,confidence:%f,requestId:%s", labels, confidence, requestId)
	if result.Reason != expectedReason {
		t.Fatalf("expected reason %s, got %s", expectedReason, result.Reason)
	}
}

func TestChainImpl_AliYun_PassLLMCheck(t *testing.T) {
	checker := mockgreen.NewMockSensitiveChecker(t)
	chain := sensitive.NewChainCheckerWithCheckers(checker)

	ctx := context.Background()
	scenario := types.ScenarioCommentDetection
	text := "test llm text"
	sessionId := "test-session-id"
	req := &types.LLMCheckRequest{Scenario: scenario, Text: text, SessionId: sessionId}
	checker.EXPECT().PassLLMCheck(ctx, req).Return(&ss_type.CheckResult{IsSensitive: false}, nil)

	result, err := chain.PassLLMCheck(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatalf("expected non-nil result, got nil")
	}
	if result.IsSensitive {
		t.Fatalf("expected non-sensitive result, got sensitive")
	}
}

func TestChainImpl_AliYun_PassLLMCheck_Sensitive(t *testing.T) {
	checker := mockgreen.NewMockSensitiveChecker(t)
	chain := sensitive.NewChainCheckerWithCheckers(checker)

	ctx := context.Background()
	scenario := types.ScenarioCommentDetection
	text := "sensitive llm text"
	sessionId := "test-session-id"
	labels := "political_content"
	riskWords := "risk words"
	expectedReason := fmt.Sprintf("label:%s,reason:%s,requestId:%s", labels, riskWords, "test-request-id")
	req := &types.LLMCheckRequest{Scenario: scenario, Text: text, SessionId: sessionId}
	checker.EXPECT().PassLLMCheck(ctx, req).Return(&ss_type.CheckResult{
		IsSensitive: true,
		Reason:      expectedReason,
	}, nil)

	result, err := chain.PassLLMCheck(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatalf("expected non-nil result, got nil")
	}
	if !result.IsSensitive {
		t.Fatalf("expected sensitive result, got non-sensitive")
	}
	if result.Reason != expectedReason {
		t.Fatalf("expected reason %s, got %s", expectedReason, result.Reason)
	}
}

func TestNewChainCheckerFromConfig(t *testing.T) {
	// Test with basic check chain (using providers that do not require external dependencies)
	cfg := &config.Config{}
	cfg.SensitiveCheck.Enable = true
	cfg.SensitiveCheck.CheckChain = []string{
		sensitive.ProviderACAutomaton,
		sensitive.ProviderAliyunGreen,
	}

	chain := sensitive.NewChainCheckerFromConfig(cfg)
	if chain == nil {
		t.Fatalf("expected non-nil chain, got nil")
	}

	// Test with unknown provider (should be ignored)
	cfg2 := &config.Config{}
	cfg2.SensitiveCheck.Enable = true
	cfg2.SensitiveCheck.CheckChain = []string{
		sensitive.ProviderACAutomaton,
		"unknown_provider",
	}

	chain2 := sensitive.NewChainCheckerFromConfig(cfg2)
	if chain2 == nil {
		t.Fatalf("expected non-nil chain, got nil")
	}

	// Test with empty CheckChain
	cfg3 := &config.Config{}
	cfg3.SensitiveCheck.Enable = true
	cfg3.SensitiveCheck.CheckChain = []string{}

	chain3 := sensitive.NewChainCheckerFromConfig(cfg3)
	if chain3 == nil {
		t.Fatalf("expected non-nil chain, got nil")
	}
}

func TestNewChainCheckerFromConfig_WithAdvanceOptions(t *testing.T) {
	// Track which providers are passed to the advance options function
	var calledProviders []string

	sensitive.RegisterAdvanceOptions(func(cfg *config.Config, provider string) []sensitive.ChainOption {
		calledProviders = append(calledProviders, provider)
		return nil
	})
	defer sensitive.RegisterAdvanceOptions(nil) // Clean up global state

	cfg := &config.Config{}
	cfg.SensitiveCheck.Enable = true
	// Use custom providers that only go through advanceOptionsFunc, not defaultCheckOpts
	cfg.SensitiveCheck.CheckChain = []string{
		"custom_advance_provider",
		"another_custom",
	}

	chain := sensitive.NewChainCheckerFromConfig(cfg)
	if chain == nil {
		t.Fatalf("expected non-nil chain, got nil")
	}

	// Verify the advance function was called for each provider
	if len(calledProviders) != 2 {
		t.Fatalf("expected advance function called 2 times, got %d", len(calledProviders))
	}
	if calledProviders[0] != "custom_advance_provider" {
		t.Fatalf("expected first provider 'custom_advance_provider', got '%s'", calledProviders[0])
	}
	if calledProviders[1] != "another_custom" {
		t.Fatalf("expected second provider 'another_custom', got '%s'", calledProviders[1])
	}
}

// TestChainImpl_PassImageStreamCheck_SeekableReader verifies that when the
// chain has multiple checkers, each checker receives the full image content.
// The reader is seekable (strings.NewReader), so the chain rewinds it before
// each checker. Both checkers read the stream and must see identical content.
func TestChainImpl_PassImageStreamCheck_SeekableReader(t *testing.T) {
	imageContent := "fake-image-bytes"

	checker1 := mockgreen.NewMockSensitiveChecker(t)
	checker2 := mockgreen.NewMockSensitiveChecker(t)
	chain := sensitive.NewChainCheckerWithCheckers(checker1, checker2)

	ctx := context.Background()
	scenario := types.ScenarioImageBaseLineCheck

	var mu sync.Mutex
	var contents []string

	checker1.EXPECT().PassImageStreamCheck(mock.Anything, scenario, mock.Anything).
		RunAndReturn(func(ctx context.Context, s types.SensitiveScenario, r io.Reader) (*ss_type.CheckResult, error) {
			b, _ := io.ReadAll(r)
			mu.Lock()
			contents = append(contents, string(b))
			mu.Unlock()
			return &ss_type.CheckResult{IsSensitive: false}, nil
		}).Once()

	checker2.EXPECT().PassImageStreamCheck(mock.Anything, scenario, mock.Anything).
		RunAndReturn(func(ctx context.Context, s types.SensitiveScenario, r io.Reader) (*ss_type.CheckResult, error) {
			b, _ := io.ReadAll(r)
			mu.Lock()
			contents = append(contents, string(b))
			mu.Unlock()
			return &ss_type.CheckResult{IsSensitive: false}, nil
		}).Once()

	result, err := chain.PassImageStreamCheck(ctx, scenario, strings.NewReader(imageContent))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsSensitive {
		t.Fatalf("expected non-sensitive result, got %+v", result)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(contents) != 2 {
		t.Fatalf("expected 2 checker calls, got %d", len(contents))
	}
	for i, got := range contents {
		if got != imageContent {
			t.Fatalf("checker %d: expected content %q, got %q", i, imageContent, got)
		}
	}
}

// TestChainImpl_PassImageStreamCheck_SeekableReader_ShortCircuit verifies that
// when the first checker detects sensitive content, the chain returns
// immediately and does not call the second checker.
func TestChainImpl_PassImageStreamCheck_SeekableReader_ShortCircuit(t *testing.T) {
	imageContent := "sensitive-image-bytes"

	checker1 := mockgreen.NewMockSensitiveChecker(t)
	checker2 := mockgreen.NewMockSensitiveChecker(t)
	chain := sensitive.NewChainCheckerWithCheckers(checker1, checker2)

	ctx := context.Background()
	scenario := types.ScenarioImageBaseLineCheck

	checker1.EXPECT().PassImageStreamCheck(mock.Anything, scenario, mock.Anything).
		RunAndReturn(func(ctx context.Context, s types.SensitiveScenario, r io.Reader) (*ss_type.CheckResult, error) {
			_, _ = io.ReadAll(r) // consume the stream
			return &ss_type.CheckResult{IsSensitive: true, Reason: "porn"}, nil
		}).Once()
	// checker2 must NOT be called
	checker2.AssertNotCalled(t, "PassImageStreamCheck", mock.Anything, mock.Anything, mock.Anything)

	result, err := chain.PassImageStreamCheck(ctx, scenario, strings.NewReader(imageContent))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || !result.IsSensitive {
		t.Fatalf("expected sensitive result, got %+v", result)
	}
	if result.Reason != "porn" {
		t.Fatalf("expected reason 'porn', got '%s'", result.Reason)
	}
}

// TestChainImpl_PassImageStreamCheck_NonSeekableReader verifies the behavior
// when the reader does not implement io.Seeker. The first checker consumes the
// stream; the second checker reads empty content. This is the known limitation
// — the test documents the boundary behavior.
func TestChainImpl_PassImageStreamCheck_NonSeekableReader(t *testing.T) {
	imageContent := "fake-image-bytes"

	checker1 := mockgreen.NewMockSensitiveChecker(t)
	checker2 := mockgreen.NewMockSensitiveChecker(t)
	chain := sensitive.NewChainCheckerWithCheckers(checker1, checker2)

	ctx := context.Background()
	scenario := types.ScenarioImageBaseLineCheck

	var mu sync.Mutex
	var contents []string

	checker1.EXPECT().PassImageStreamCheck(mock.Anything, scenario, mock.Anything).
		RunAndReturn(func(ctx context.Context, s types.SensitiveScenario, r io.Reader) (*ss_type.CheckResult, error) {
			b, _ := io.ReadAll(r)
			mu.Lock()
			contents = append(contents, string(b))
			mu.Unlock()
			return &ss_type.CheckResult{IsSensitive: false}, nil
		}).Once()

	checker2.EXPECT().PassImageStreamCheck(mock.Anything, scenario, mock.Anything).
		RunAndReturn(func(ctx context.Context, s types.SensitiveScenario, r io.Reader) (*ss_type.CheckResult, error) {
			b, _ := io.ReadAll(r)
			mu.Lock()
			contents = append(contents, string(b))
			mu.Unlock()
			return &ss_type.CheckResult{IsSensitive: false}, nil
		}).Once()

	// Wrap in a struct that only exposes Read, hiding Seek.
	nonSeekable := struct{ io.Reader }{strings.NewReader(imageContent)}

	result, err := chain.PassImageStreamCheck(ctx, scenario, nonSeekable)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsSensitive {
		t.Fatalf("expected non-sensitive result, got %+v", result)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(contents) != 2 {
		t.Fatalf("expected 2 checker calls, got %d", len(contents))
	}
	// First checker gets full content
	if contents[0] != imageContent {
		t.Fatalf("checker 0: expected %q, got %q", imageContent, contents[0])
	}
	// Second checker gets empty content (non-seekable, stream consumed)
	if contents[1] != "" {
		t.Fatalf("checker 1: expected empty (non-seekable stream consumed), got %q", contents[1])
	}
}

// TestChainImpl_PassImageStreamCheck_SingleChecker verifies that a single
// checker works correctly with a seekable reader (no unnecessary seek issues).
func TestChainImpl_PassImageStreamCheck_SingleChecker(t *testing.T) {
	imageContent := "single-checker-bytes"

	checker1 := mockgreen.NewMockSensitiveChecker(t)
	chain := sensitive.NewChainCheckerWithCheckers(checker1)

	ctx := context.Background()
	scenario := types.ScenarioImageBaseLineCheck

	checker1.EXPECT().PassImageStreamCheck(mock.Anything, scenario, mock.Anything).
		RunAndReturn(func(ctx context.Context, s types.SensitiveScenario, r io.Reader) (*ss_type.CheckResult, error) {
			b, _ := io.ReadAll(r)
			if string(b) != imageContent {
				t.Fatalf("expected %q, got %q", imageContent, string(b))
			}
			return &ss_type.CheckResult{IsSensitive: false}, nil
		}).Once()

	result, err := chain.PassImageStreamCheck(ctx, scenario, strings.NewReader(imageContent))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.IsSensitive {
		t.Fatalf("expected non-sensitive result, got %+v", result)
	}
}
