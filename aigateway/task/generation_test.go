package task

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	taskprocessor "opencsg.com/csghub-server/aigateway/task/processor"
	"opencsg.com/csghub-server/builder/store/database"
)

type fakeGenerationProcessor struct {
	resourceType string
}

func (p *fakeGenerationProcessor) ResourceType() string { return p.resourceType }

func (p *fakeGenerationProcessor) Refresh(_ context.Context, _ taskprocessor.GenerationRef) (*taskprocessor.GenerationStatus, error) {
	return nil, nil
}

func TestBuildProcessorMapReturnsEmptyForEmptyInput(t *testing.T) {
	require.Empty(t, buildProcessorMap(nil))
	require.Empty(t, buildProcessorMap([]taskprocessor.ResourceProcessor{}))
}

func TestBuildProcessorMapIndexesByResourceType(t *testing.T) {
	video := &fakeGenerationProcessor{resourceType: "video"}
	audio := &fakeGenerationProcessor{resourceType: "audio"}

	got := buildProcessorMap([]taskprocessor.ResourceProcessor{video, audio})

	require.Len(t, got, 2)
	require.Same(t, video, got["video"])
	require.Same(t, audio, got["audio"])
}

func TestBuildProcessorMapSkipsNilAndBlankEntries(t *testing.T) {
	video := &fakeGenerationProcessor{resourceType: "video"}
	blank := &fakeGenerationProcessor{resourceType: ""}
	whitespace := &fakeGenerationProcessor{resourceType: "   \t\n"}

	got := buildProcessorMap([]taskprocessor.ResourceProcessor{
		nil, video, blank, whitespace,
	})

	require.Len(t, got, 1, "nil and blank/whitespace ResourceType entries must be dropped")
	require.Same(t, video, got["video"])
}

func TestBuildProcessorMapLastEntryWinsOnDuplicateResourceType(t *testing.T) {
	first := &fakeGenerationProcessor{resourceType: "video"}
	second := &fakeGenerationProcessor{resourceType: "video"}

	got := buildProcessorMap([]taskprocessor.ResourceProcessor{first, second})

	require.Len(t, got, 1)
	require.Same(t, second, got["video"], "later entry must overwrite earlier entry of same type")
}

func TestApplyGenerationStatusIsNilSafe(t *testing.T) {
	require.NotPanics(t, func() { applyGenerationStatus(nil, &taskprocessor.GenerationStatus{}) })
	require.NotPanics(t, func() { applyGenerationStatus(&database.AIGeneration{}, nil) })
	require.NotPanics(t, func() { applyGenerationStatus(nil, nil) })
}

func TestApplyGenerationStatusEmptyStatusLeavesGenerationUnchanged(t *testing.T) {
	started := time.Now()
	gen := &database.AIGeneration{
		Status:     "queued",
		Progress:   "0.0",
		FailReason: "old reason",
		StartedAt:  &started,
	}

	applyGenerationStatus(gen, &taskprocessor.GenerationStatus{})

	require.Equal(t, "queued", gen.Status)
	require.Equal(t, "0.0", gen.Progress)
	require.Equal(t, "old reason", gen.FailReason)
	require.Equal(t, &started, gen.StartedAt)
	require.Nil(t, gen.FinishedAt)
}

func TestApplyGenerationStatusOverwritesAllNonEmptyFields(t *testing.T) {
	started := time.Now().Add(-2 * time.Minute)
	finished := time.Now()
	gen := &database.AIGeneration{}

	status := &taskprocessor.GenerationStatus{
		Status:     "completed",
		Progress:   "1.0",
		FailReason: "ok",
		StartedAt:  &started,
		FinishedAt: &finished,
	}

	applyGenerationStatus(gen, status)

	require.Equal(t, "completed", gen.Status)
	require.Equal(t, "1.0", gen.Progress)
	require.Equal(t, "ok", gen.FailReason)
	require.Equal(t, &started, gen.StartedAt)
	require.Equal(t, &finished, gen.FinishedAt)
}

func TestApplyGenerationStatusSkipsNilTimeFields(t *testing.T) {
	started := time.Now().Add(-time.Minute)
	gen := &database.AIGeneration{
		Status:    "in_progress",
		StartedAt: &started,
	}

	// status carries nil StartedAt → must NOT clobber the existing one.
	status := &taskprocessor.GenerationStatus{
		Status:    "completed",
		StartedAt: nil,
	}

	applyGenerationStatus(gen, status)

	require.Equal(t, "completed", gen.Status)
	require.Equal(t, &started, gen.StartedAt, "non-nil existing StartedAt must be preserved")
}

func TestGenerationRefFromGenerationCopiesAllExportedFields(t *testing.T) {
	now := time.Now()
	providerMeta := map[string]any{"k": "v"}
	gen := database.AIGeneration{
		ID:                 42,
		ResourceID:         "res-42",
		ProviderResourceID: "prov-42",
		ProviderMetadata:   providerMeta,
		UpstreamID:         7,
		ModelID:            "model-42",
		Status:             "in_progress",
		StartedAt:          &now,
		FinishedAt:         nil,
	}

	ref := generationRefFromGeneration(gen)

	require.Equal(t, int64(42), ref.ID)
	require.Equal(t, "res-42", ref.ResourceID)
	require.Equal(t, "prov-42", ref.ProviderResourceID)
	require.Equal(t, providerMeta, ref.ProviderMetadata)
	require.Equal(t, int64(7), ref.UpstreamID)
	require.Equal(t, "model-42", ref.ModelID)
	require.Equal(t, "in_progress", ref.Status)
	require.Equal(t, &now, ref.StartedAt)
	require.Nil(t, ref.FinishedAt)
}

func TestGenerationRefFromGenerationPreservesNilPointers(t *testing.T) {
	ref := generationRefFromGeneration(database.AIGeneration{})

	require.Nil(t, ref.StartedAt)
	require.Nil(t, ref.FinishedAt)
	require.Nil(t, ref.ProviderMetadata)
}
