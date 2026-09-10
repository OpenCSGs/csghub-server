package component

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

const (
	policyBucket   = "opencsg-public-resource"
	policyEndpoint = "cdn.example.com"
	policyAudioURL = "https://opencsg-public-resource.cdn.example.com/audio/11111111-1111-4111-8111-111111111111.mp3"
	policyVideoURL = "https://opencsg-public-resource.cdn.example.com/video/22222222-2222-4222-8222-222222222222.mp4"
	policyImageURL = "https://opencsg-public-resource.cdn.example.com/image/33333333-3333-4333-8333-333333333333.png"
)

func newTestPolicy(t *testing.T, enabled bool, images SensitiveComponent, media MediaModerationComponent) CommentMediaPolicy {
	t.Helper()
	return NewCommentMediaPolicy(enabled, policyBucket, policyEndpoint, 3, images, media)
}

func TestCommentMediaPolicy_DisabledReturnsPass(t *testing.T) {
	policy := newTestPolicy(t, false, nil, nil)
	decision, err := policy.Check(context.Background(), `<audio src="https://external.example.com/not-an-uuid.mp3"></audio>`)
	require.NoError(t, err)
	assert.Equal(t, types.MediaModerationDecisionPass, decision.Decision)
}

func TestCommentMediaPolicy_ImageDisabledIgnoresExternalImages(t *testing.T) {
	images := mockcomponent.NewMockSensitiveComponent(t)
	media := mockcomponent.NewMockMediaModerationComponent(t)
	cfg := &config.Config{}
	cfg.SensitiveCheck.Enable = true
	cfg.SensitiveCheck.ImageCheckEnable = false
	cfg.SensitiveCheck.MediaModerationEnable = true
	cfg.S3.PublicBucket = policyBucket
	cfg.S3.Endpoint = policyEndpoint
	policy := NewCommentMediaPolicyFromConfig(cfg, images, media)

	media.EXPECT().Check(context.Background(), mock.MatchedBy(func(refs []types.MediaRef) bool {
		return len(refs) == 1 && refs[0].URL == policyAudioURL
	})).Return(types.CommentMediaDecision{Decision: types.MediaModerationDecisionPass}, nil)

	decision, err := policy.Check(context.Background(),
		`![external](https://external.example.com/not-an-uuid.png)<audio src="`+policyAudioURL+`"></audio>`)
	require.NoError(t, err)
	assert.Equal(t, types.MediaModerationDecisionPass, decision.Decision)
	images.AssertNotCalled(t, "CheckImageURL")
}

func TestCommentMediaPolicy_MediaDisabledIgnoresExternalAudio(t *testing.T) {
	images := mockcomponent.NewMockSensitiveComponent(t)
	media := mockcomponent.NewMockMediaModerationComponent(t)
	cfg := &config.Config{}
	cfg.SensitiveCheck.Enable = true
	cfg.SensitiveCheck.ImageCheckEnable = true
	cfg.SensitiveCheck.MediaModerationEnable = false
	cfg.S3.PublicBucket = policyBucket
	cfg.S3.Endpoint = policyEndpoint
	policy := NewCommentMediaPolicyFromConfig(cfg, images, media)

	images.EXPECT().CheckImageURL(context.Background(), types.ScenarioImageBaseLineCheck, policyImageURL).
		Return(true, nil)

	decision, err := policy.Check(context.Background(),
		`![image](`+policyImageURL+`)<audio src="https://external.example.com/not-an-uuid.mp3"></audio>`)
	require.NoError(t, err)
	assert.Equal(t, types.MediaModerationDecisionPass, decision.Decision)
	media.AssertNotCalled(t, "Check")
}

func TestCommentMediaPolicy_ImageCheckDisabledSkipsImages(t *testing.T) {
	images := mockcomponent.NewMockSensitiveComponent(t)
	media := mockcomponent.NewMockMediaModerationComponent(t)
	cfg := &config.Config{}
	cfg.SensitiveCheck.Enable = true
	cfg.SensitiveCheck.MediaModerationEnable = true
	cfg.SensitiveCheck.ImageCheckEnable = false
	cfg.S3.PublicBucket = policyBucket
	cfg.S3.Endpoint = policyEndpoint
	policy := NewCommentMediaPolicyFromConfig(cfg, images, media)

	decision, err := policy.Check(context.Background(),
		`![a](`+policyImageURL+`)`)
	require.NoError(t, err)
	assert.Equal(t, types.MediaModerationDecisionPass, decision.Decision)
}

func TestCommentMediaPolicy_ImageCheckRunsWhenAsyncMediaDisabled(t *testing.T) {
	images := mockcomponent.NewMockSensitiveComponent(t)
	media := mockcomponent.NewMockMediaModerationComponent(t)
	cfg := &config.Config{}
	cfg.SensitiveCheck.Enable = true
	cfg.SensitiveCheck.ImageCheckEnable = true
	cfg.SensitiveCheck.MediaModerationEnable = false
	cfg.S3.PublicBucket = policyBucket
	cfg.S3.Endpoint = policyEndpoint
	policy := NewCommentMediaPolicyFromConfig(cfg, images, media)

	images.EXPECT().CheckImageURL(context.Background(), types.ScenarioImageBaseLineCheck, policyImageURL).
		Return(true, nil)

	decision, err := policy.Check(context.Background(),
		`![a](`+policyImageURL+`)<audio src="`+policyAudioURL+`"></audio>`)
	require.NoError(t, err)
	assert.Equal(t, types.MediaModerationDecisionPass, decision.Decision)
	media.AssertNotCalled(t, "Check")
}

func TestCommentMediaPolicy_OverallSensitiveCheckDisabledDisablesMedia(t *testing.T) {
	images := mockcomponent.NewMockSensitiveComponent(t)
	media := mockcomponent.NewMockMediaModerationComponent(t)
	cfg := &config.Config{}
	cfg.SensitiveCheck.Enable = false
	cfg.SensitiveCheck.MediaModerationEnable = true
	cfg.S3.PublicBucket = policyBucket
	cfg.S3.Endpoint = policyEndpoint
	policy := NewCommentMediaPolicyFromConfig(cfg, images, media)

	decision, err := policy.Check(context.Background(), `<audio src="`+policyAudioURL+`"></audio>`)
	require.NoError(t, err)
	assert.Equal(t, types.MediaModerationDecisionPass, decision.Decision)
}

func TestCommentMediaPolicy_ImageSensitiveRejects(t *testing.T) {
	images := mockcomponent.NewMockSensitiveComponent(t)
	media := mockcomponent.NewMockMediaModerationComponent(t)
	policy := newTestPolicy(t, true, images, media)

	images.EXPECT().CheckImageURL(context.Background(), types.ScenarioImageBaseLineCheck, policyImageURL).
		Return(false, nil)

	decision, err := policy.Check(context.Background(), `![a](`+policyImageURL+`)`)
	require.NoError(t, err)
	assert.Equal(t, types.MediaModerationDecisionReject, decision.Decision)
}

func TestCommentMediaPolicy_ImageCheckErrorFailsClosed(t *testing.T) {
	images := mockcomponent.NewMockSensitiveComponent(t)
	media := mockcomponent.NewMockMediaModerationComponent(t)
	policy := newTestPolicy(t, true, images, media)

	images.EXPECT().CheckImageURL(context.Background(), types.ScenarioImageBaseLineCheck, policyImageURL).
		Return(false, errors.New("image check offline"))

	decision, err := policy.Check(context.Background(), `![a](`+policyImageURL+`)`)
	require.Error(t, err)
	assert.Equal(t, types.MediaModerationDecisionError, decision.Decision)
}

func TestCommentMediaPolicy_AudioVideoPending(t *testing.T) {
	images := mockcomponent.NewMockSensitiveComponent(t)
	media := mockcomponent.NewMockMediaModerationComponent(t)
	policy := newTestPolicy(t, true, images, media)

	media.EXPECT().Check(context.Background(), mock.MatchedBy(func(refs []types.MediaRef) bool {
		return len(refs) == 1 && refs[0].URL == policyAudioURL && refs[0].Type == types.MediaTypeAudio
	})).
		Return(types.CommentMediaDecision{
			Decision: types.MediaModerationDecisionPending,
			Items:    []types.CommentMediaItem{{DataID: "d1", TaskID: "t1", MediaType: types.MediaTypeAudio}},
		}, nil)

	decision, err := policy.Check(context.Background(), `<audio src="`+policyAudioURL+`"></audio>`)
	require.NoError(t, err)
	assert.Equal(t, types.MediaModerationDecisionPending, decision.Decision)
	require.Len(t, decision.Items, 1)
}

func TestCommentMediaPolicy_AllPass(t *testing.T) {
	images := mockcomponent.NewMockSensitiveComponent(t)
	media := mockcomponent.NewMockMediaModerationComponent(t)
	policy := newTestPolicy(t, true, images, media)

	images.EXPECT().CheckImageURL(context.Background(), types.ScenarioImageBaseLineCheck, policyImageURL).
		Return(true, nil)
	media.EXPECT().Check(context.Background(), mock.MatchedBy(func(refs []types.MediaRef) bool {
		return len(refs) == 1 && refs[0].URL == policyVideoURL && refs[0].Type == types.MediaTypeVideo
	})).
		Return(types.CommentMediaDecision{Decision: types.MediaModerationDecisionPass}, nil)

	decision, err := policy.Check(context.Background(), `![a](`+policyImageURL+`)<video src="`+policyVideoURL+`"></video>`)
	require.NoError(t, err)
	assert.Equal(t, types.MediaModerationDecisionPass, decision.Decision)
}

func TestCommentMediaPolicy_TooManyRefsErrors(t *testing.T) {
	policy := NewCommentMediaPolicy(true, policyBucket, policyEndpoint, 1, nil, nil)
	content := `<audio src="` + policyAudioURL + `"></audio><video src="` + policyVideoURL + `"></video>`
	decision, err := policy.Check(context.Background(), content)
	require.Error(t, err)
	assert.Equal(t, types.MediaModerationDecisionError, decision.Decision)
}

func TestCommentMediaPolicy_ExternalAudioRejected(t *testing.T) {
	policy := newTestPolicy(t, true, nil, nil)
	decision, err := policy.Check(context.Background(), `<audio src="https://other.example/a.mp3"></audio>`)
	require.Error(t, err)
	assert.Equal(t, types.MediaModerationDecisionError, decision.Decision)
}

func TestCommentMediaPolicy_ImageLimitCheckedBeforeExternalCalls(t *testing.T) {
	images := mockcomponent.NewMockSensitiveComponent(t)
	media := mockcomponent.NewMockMediaModerationComponent(t)
	policy := newCommentMediaPolicy(true, true, policyBucket, policyEndpoint, 1, 1, 2, images, media)
	content := `![a](` + policyImageURL + `)![b](https://opencsg-public-resource.cdn.example.com/image/44444444-4444-4444-8444-444444444444.png)`

	decision, err := policy.Check(context.Background(), content)
	require.Error(t, err)
	assert.Equal(t, types.MediaModerationDecisionError, decision.Decision)
	images.AssertNotCalled(t, "CheckImageURL")
	media.AssertNotCalled(t, "Check")
}

func TestCommentMediaPolicy_TotalLimitCheckedBeforeExternalCalls(t *testing.T) {
	images := mockcomponent.NewMockSensitiveComponent(t)
	media := mockcomponent.NewMockMediaModerationComponent(t)
	policy := newCommentMediaPolicy(true, true, policyBucket, policyEndpoint, 2, 2, 1, images, media)
	content := `![a](` + policyImageURL + `)<audio src="` + policyAudioURL + `"></audio>`

	decision, err := policy.Check(context.Background(), content)
	require.Error(t, err)
	assert.Equal(t, types.MediaModerationDecisionError, decision.Decision)
	images.AssertNotCalled(t, "CheckImageURL")
	media.AssertNotCalled(t, "Check")
}

func TestCommentMediaPolicy_CheckUpdateRejectsAudioBeforeSubmission(t *testing.T) {
	images := mockcomponent.NewMockSensitiveComponent(t)
	media := mockcomponent.NewMockMediaModerationComponent(t)
	policy := newTestPolicy(t, true, images, media)

	media.EXPECT().CheckExisting(context.Background(), mock.Anything).Return(
		types.CommentMediaDecision{Decision: types.MediaModerationDecisionPending}, nil)
	decision, err := policy.CheckUpdate(context.Background(), `<audio src="`+policyAudioURL+`"></audio>`)
	require.NoError(t, err)
	assert.Equal(t, types.MediaModerationDecisionPending, decision.Decision)
	media.AssertNotCalled(t, "Check")
}

func TestCommentMediaPolicy_CheckUpdateAllowsApprovedAudio(t *testing.T) {
	images := mockcomponent.NewMockSensitiveComponent(t)
	media := mockcomponent.NewMockMediaModerationComponent(t)
	policy := newTestPolicy(t, true, images, media)
	media.EXPECT().CheckExisting(context.Background(), mock.Anything).Return(
		types.CommentMediaDecision{Decision: types.MediaModerationDecisionPass}, nil)
	decision, err := policy.CheckUpdate(context.Background(), `<audio src="`+policyAudioURL+`"></audio> corrected text`)
	require.NoError(t, err)
	assert.Equal(t, types.MediaModerationDecisionPass, decision.Decision)
	media.AssertNotCalled(t, "Check")
}

func TestCommentMediaPolicy_ValidationErrorIsIdentifiable(t *testing.T) {
	policy := newCommentMediaPolicy(true, true, policyBucket, policyEndpoint, 1, 1, 1, nil, nil)
	content := `<audio src="` + policyAudioURL + `"></audio><video src="` + policyVideoURL + `"></video>`

	decision, err := policy.Check(context.Background(), content)
	require.ErrorIs(t, err, ErrInvalidCommentMedia)
	assert.Equal(t, types.MediaModerationDecisionError, decision.Decision)
}
