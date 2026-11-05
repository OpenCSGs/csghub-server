package component

import (
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestResolveDeployTask_ASR(t *testing.T) {
	tests := []struct {
		name string
		tags []database.Tag
	}{
		{
			name: "canonical hf task",
			tags: []database.Tag{{Name: string(types.AutomaticSpeechRecognition)}},
		},
		{
			name: "legacy auto speech recognition tag",
			tags: []database.Tag{{Name: string(types.AutoSpeechRecognition)}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, string(types.AutomaticSpeechRecognition), resolveDeployTask(tt.tags, string(types.LlamaCpp)))
		})
	}
}

func TestResolveDeployTask_Image2Image(t *testing.T) {
	task := resolveDeployTask([]database.Tag{{Name: string(types.Image2Image)}}, string(types.LlamaCpp))

	require.Equal(t, string(types.Image2Image), task)
}

func TestResolveDeployTask_TextRanking(t *testing.T) {
	task := resolveDeployTask([]database.Tag{{Name: string(types.TextRanking)}}, string(types.LlamaCpp))

	require.Equal(t, string(types.TextRanking), task)
}

func TestResolveDeployTask_TextToAudio(t *testing.T) {
	task := resolveDeployTask([]database.Tag{{Name: string(types.TextToAudio)}}, string(types.LlamaCpp))

	require.Equal(t, string(types.TextToAudio), task)
}

func TestResolveDeployTask_AudioDrivenVideo(t *testing.T) {
	tasks := []types.PipelineTask{
		types.AudioText2Video,
		types.AudioImageText2Video,
		types.AudioDrivenVideoContinuation,
	}
	for _, task := range tasks {
		t.Run(string(task), func(t *testing.T) {
			tags := []database.Tag{{Name: string(task)}}

			require.Equal(t, string(task), resolveDeployTask(tags, string(types.LlamaCpp)))
		})
	}
}

func TestResolveDeployTask_OpticalCharacterRecognition(t *testing.T) {
	task := resolveDeployTask([]database.Tag{{Name: string(types.OpticalCharacterRecognition)}}, string(types.LlamaCpp))

	require.Equal(t, string(types.OpticalCharacterRecognition), task)
}

func TestResolveDeployTask_Text2VideoSynthesis(t *testing.T) {
	tests := []struct {
		name string
		tags []database.Tag
	}{
		{
			name: "platform text-to-video tag",
			tags: []database.Tag{{Name: string(types.Text2Video)}},
		},
		{
			name: "upstream text-to-video-synthesis tag",
			tags: []database.Tag{{Name: string(types.Text2VideoSynthesis)}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, string(types.Text2Video), resolveDeployTask(tt.tags, string(types.LlamaCpp)))
		})
	}
}

func TestResolveDeployTask_DecisionTags(t *testing.T) {
	tests := []struct {
		name string
		tags []database.Tag
		want string
		// engine is the deploy runtime framework; empty means llama.cpp, the
		// engine most decision models are deployed on.
		engine string
	}{
		{
			name: "decision-model tag",
			tags: []database.Tag{{Name: "decision-model"}},
			want: string(types.TypedDecisions),
		},
		{
			name: "decision-model with feature-extraction tag",
			tags: []database.Tag{{Name: "decision-model"}, {Name: string(types.FeatureExtraction)}},
			want: string(types.TypedDecisions),
		},
		{
			name: "decision-model with image-text-to-text tag",
			tags: []database.Tag{{Name: "decision-model"}, {Name: string(types.ImageText2Text)}},
			want: string(types.TypedDecisions),
		},
		{
			name: "decision-model with text-classification tag",
			tags: []database.Tag{{Name: "decision-model"}, {Name: "text-classification"}},
			want: string(types.TypedDecisions),
		},
		{
			name: "decision-model with zero-shot-classification tag",
			tags: []database.Tag{{Name: "decision-model"}, {Name: "zero-shot-classification"}},
			want: string(types.TypedDecisions),
		},
		{
			name: "decision tag",
			tags: []database.Tag{{Name: "decision"}},
			want: string(types.TypedDecisions),
		},
		{
			name: "decision tag with image-text-to-text tag",
			tags: []database.Tag{{Name: "decision"}, {Name: string(types.ImageText2Text)}},
			want: string(types.TypedDecisions),
		},
		{
			name: "decision tag with gguf tags",
			tags: []database.Tag{{Name: "decision"}, {Name: "gguf"}, {Name: "llama.cpp"}},
			want: string(types.TypedDecisions),
		},
		{
			name: "system-one with jev tag",
			tags: []database.Tag{{Name: "system-one"}, {Name: "jev"}},
			want: string(types.TypedDecisions),
		},
		{
			name: "system-one with laya tag",
			tags: []database.Tag{{Name: "system-one"}, {Name: "laya"}},
			want: string(types.TypedDecisions),
		},
		{
			name: "system-one with laya wins over text-generation tag",
			tags: []database.Tag{{Name: "system-one"}, {Name: "laya"}, {Name: string(types.TextGeneration)}},
			want: string(types.TypedDecisions),
		},
		{
			name: "system-one with jev and system-two tags",
			tags: []database.Tag{{Name: "system-one"}, {Name: "system-two"}, {Name: "jev"}},
			want: string(types.TypedDecisions),
		},
		{
			name: "system-one without jev or laya",
			tags: []database.Tag{{Name: "system-one"}},
			want: string(types.TextGeneration),
		},
		{
			name: "jev without system-one",
			tags: []database.Tag{{Name: "jev"}},
			want: string(types.TextGeneration),
		},
		{
			name: "system-2 decision model is not matched",
			tags: []database.Tag{{Name: "typed-decisions"}, {Name: "jev"}, {Name: "decision-index"}, {Name: "system-two"}},
			want: string(types.TextGeneration),
		},
		{
			name: "decision-index alone is not matched",
			tags: []database.Tag{{Name: "decision-index"}},
			want: string(types.TextGeneration),
		},
		{
			name: "text-generation tag",
			tags: []database.Tag{{Name: string(types.TextGeneration)}},
			want: string(types.TextGeneration),
		},
		{
			name: "image-text-to-text tag",
			tags: []database.Tag{{Name: string(types.ImageText2Text)}},
			want: string(types.ImageText2Text),
		},
		{
			name: "no tags falls back to text-generation",
			want: string(types.TextGeneration),
		},
		{
			name:   "decision-model on an engine without SystemOne keeps built-in task",
			tags:   []database.Tag{{Name: "decision-model"}},
			want:   string(types.TextGeneration),
			engine: "vllm",
		},
		{
			name:   "decision-model with image-text-to-text on sglang keeps built-in task",
			tags:   []database.Tag{{Name: "decision-model"}, {Name: string(types.ImageText2Text)}},
			want:   string(types.ImageText2Text),
			engine: "sglang",
		},
		{
			name:   "system-one with laya on an engine without SystemOne keeps built-in task",
			tags:   []database.Tag{{Name: "system-one"}, {Name: "laya"}},
			want:   string(types.TextGeneration),
			engine: "vllm",
		},
		{
			name:   "laya engine without decision tags keeps built-in task",
			tags:   []database.Tag{{Name: string(types.TextGeneration)}},
			want:   string(types.TextGeneration),
			engine: "laya",
		},
		{
			name: "decision tag with text-generation tag keeps chat task",
			tags: []database.Tag{{Name: "decision"}, {Name: string(types.TextGeneration)}},
			want: string(types.TextGeneration),
		},
		{
			name: "decision-model with text-generation tag still wins",
			tags: []database.Tag{{Name: "decision-model"}, {Name: string(types.TextGeneration)}},
			want: string(types.TypedDecisions),
		},
	}

	for _, tt := range tests {
		engine := tt.engine
		if engine == "" {
			engine = string(types.LlamaCpp)
		}
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, resolveDeployTask(tt.tags, engine))
		})
	}
}
