package sensitive_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/builder/sensitive/internal"
	"opencsg.com/csghub-server/common/types"
)

func initTestACAutomaton() sensitive.SensitiveChecker {
	tagMap := map[int]string{
		0: "porn",
		1: "violence",
		2: "politics",
	}
	words := []string{
		"word1",
		"word2",
		"word3",
	}
	data := &internal.SensitiveWordData{
		TagMap: tagMap,
		Words:  words,
	}
	return sensitive.NewACAutomation(data)
}

func TestNewACAutomaton(t *testing.T) {
	// Test case 1: Normal initialization

	checker := initTestACAutomaton()
	assert.NotNil(t, checker)

	// Test case 2: Multiple calls should return different instances (MutableAC is not a singleton)
	checker2 := initTestACAutomaton()
	assert.NotNil(t, checker2)

	// They should be same instances
	assert.Equal(t, checker, checker2)
}

func TestAC_PassTextCheck(t *testing.T) {
	checker := initTestACAutomaton()
	ctx := context.Background()

	// Test case 1: Text contains sensitive word
	result, err := checker.PassTextCheck(ctx, "", "test word1")
	assert.NoError(t, err)
	assert.True(t, result.IsSensitive)
	assert.Contains(t, result.Reason, "porn")
	assert.Contains(t, result.Reason, "word1")

	// Test case 2: Text contains another sensitive word
	result, err = checker.PassTextCheck(ctx, "", "test word2")
	assert.NoError(t, err)
	assert.True(t, result.IsSensitive)
	assert.Contains(t, result.Reason, "violence")
	assert.Contains(t, result.Reason, "word2")

	// Test case 3: Text contains no sensitive words
	result, err = checker.PassTextCheck(ctx, "", "test word")
	assert.NoError(t, err)
	assert.False(t, result.IsSensitive)
	assert.Empty(t, result.Reason)

	// Test case 4: Text with cleaned characters
	result, err = checker.PassTextCheck(ctx, "", "test word 3")
	assert.NoError(t, err)
	assert.True(t, result.IsSensitive)
}

func TestAC_PassLLMCheck(t *testing.T) {
	checker := initTestACAutomaton()
	ctx := context.Background()

	// Test case 1: LLM query with sensitive word
	result, err := checker.PassLLMCheck(ctx, types.ScenarioLLMQueryModeration, "test word1", "", "")
	assert.NoError(t, err)
	assert.True(t, result.IsSensitive)

	// Test case 2: LLM response with sensitive word
	result, err = checker.PassLLMCheck(ctx, types.ScenarioLLMResModeration, "test word2", "", "")
	assert.NoError(t, err)
	assert.True(t, result.IsSensitive)

	// Test case 3: LLM query with no sensitive words
	result, err = checker.PassLLMCheck(ctx, types.ScenarioLLMQueryModeration, "test word", "", "")
	assert.NoError(t, err)
	assert.False(t, result.IsSensitive)

	// Test case 4: Unsupported scenario
	result, err = checker.PassLLMCheck(ctx, "unsupported", "test word3", "", "")
	assert.NoError(t, err)
	assert.False(t, result.IsSensitive)
}

func TestAC_PassImageCheck(t *testing.T) {
	checker := initTestACAutomaton()
	ctx := context.Background()

	// Test case: Image check should return non-sensitive (not implemented)
	result, err := checker.PassImageCheck(ctx, "", "", "")
	assert.NoError(t, err)
	assert.False(t, result.IsSensitive)
}

func TestAC_PassImageURLCheck(t *testing.T) {
	checker := initTestACAutomaton()
	ctx := context.Background()

	// Test case: Image URL check should return non-sensitive (not implemented)
	result, err := checker.PassImageURLCheck(ctx, "", "")
	assert.NoError(t, err)
	assert.False(t, result.IsSensitive)
}
