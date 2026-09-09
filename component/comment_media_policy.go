package component

import (
	"context"
	"errors"
	"fmt"

	utils "opencsg.com/csghub-server/common/utils/common"

	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

// CommentMediaPolicy is the single media-safety gate before a comment write.
// Images are checked synchronously; audio/video are submitted for asynchronous
// moderation. When the feature is disabled it returns pass with no items.
type CommentMediaPolicy interface {
	Check(ctx context.Context, content string) (types.CommentMediaDecision, error)
	// CheckUpdate validates the complete edited comment without submitting new
	// audio/video work. Audio/video may be retained or added only when every UUID
	// resource already has a cached passing result.
	CheckUpdate(ctx context.Context, content string) (types.CommentMediaDecision, error)
}

// ErrInvalidCommentMedia marks deterministic client-input failures such as an
// unsupported URL or an exceeded media-count limit.
var ErrInvalidCommentMedia = errors.New("invalid comment media")

type commentMediaPolicyImpl struct {
	imagesEnabled  bool
	mediaEnabled   bool
	publicBucket   string
	publicEndpoint string
	maxAVRefs      int
	maxImageRefs   int
	maxTotalRefs   int
	images         SensitiveComponent
	media          MediaModerationComponent
}

const (
	defaultCommentMediaMaxAVRefs    = 3
	defaultCommentMediaMaxImageRefs = 5
	defaultCommentMediaMaxTotalRefs = 5
)

func NewCommentMediaPolicy(
	enabled bool,
	publicBucket, publicEndpoint string,
	maxRefs int,
	images SensitiveComponent,
	media MediaModerationComponent,
) CommentMediaPolicy {
	return newCommentMediaPolicy(enabled, enabled, publicBucket, publicEndpoint, maxRefs, maxRefs, maxRefs, images, media)
}

func newCommentMediaPolicy(
	imagesEnabled, mediaEnabled bool,
	publicBucket, publicEndpoint string,
	maxAVRefs, maxImageRefs, maxTotalRefs int,
	images SensitiveComponent,
	media MediaModerationComponent,
) CommentMediaPolicy {
	maxAVRefs = positiveOrDefault(maxAVRefs, defaultCommentMediaMaxAVRefs)
	maxImageRefs = positiveOrDefault(maxImageRefs, defaultCommentMediaMaxImageRefs)
	maxTotalRefs = positiveOrDefault(maxTotalRefs, defaultCommentMediaMaxTotalRefs)
	return &commentMediaPolicyImpl{
		imagesEnabled: imagesEnabled, mediaEnabled: mediaEnabled,
		publicBucket: publicBucket, publicEndpoint: publicEndpoint,
		maxAVRefs: maxAVRefs, maxImageRefs: maxImageRefs, maxTotalRefs: maxTotalRefs,
		images: images, media: media,
	}
}

func positiveOrDefault(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

// NewCommentMediaPolicyFromConfig composes independent synchronous image and
// asynchronous audio/video checks under the overall sensitive-check switch.
func NewCommentMediaPolicyFromConfig(cfg *config.Config, images SensitiveComponent, media MediaModerationComponent) CommentMediaPolicy {
	imagesEnabled := cfg.SensitiveCheck.Enable && cfg.SensitiveCheck.ImageCheckEnable
	mediaEnabled := cfg.SensitiveCheck.Enable && cfg.SensitiveCheck.MediaModerationEnable
	return newCommentMediaPolicy(
		imagesEnabled,
		mediaEnabled,
		cfg.S3.PublicBucket, cfg.S3.Endpoint,
		cfg.SensitiveCheck.MediaModerationMaxRefsPerComment,
		cfg.SensitiveCheck.ImageCheckMaxRefsPerComment,
		cfg.SensitiveCheck.CommentMediaMaxRefsPerComment,
		images, media,
	)
}

func (p *commentMediaPolicyImpl) Check(ctx context.Context, content string) (types.CommentMediaDecision, error) {
	if !p.imagesEnabled && !p.mediaEnabled {
		return types.CommentMediaDecision{Decision: types.MediaModerationDecisionPass}, nil
	}
	imageRefs, avRefs, err := p.parseAndValidate(content)
	if err != nil {
		return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, fmt.Errorf("%w: %v", ErrInvalidCommentMedia, err)
	}
	if p.imagesEnabled {
		decision, err := p.checkImages(ctx, imageRefs)
		if err != nil || decision.Decision == types.MediaModerationDecisionReject {
			return decision, err
		}
	}
	if !p.mediaEnabled || len(avRefs) == 0 {
		return types.CommentMediaDecision{Decision: types.MediaModerationDecisionPass}, nil
	}
	if p.media == nil {
		return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, fmt.Errorf("audio/video checker is not configured")
	}
	return p.media.Check(ctx, avRefs)
}

func (p *commentMediaPolicyImpl) CheckUpdate(ctx context.Context, content string) (types.CommentMediaDecision, error) {
	if !p.imagesEnabled && !p.mediaEnabled {
		return types.CommentMediaDecision{Decision: types.MediaModerationDecisionPass}, nil
	}
	imageRefs, avRefs, err := p.parseAndValidate(content)
	if err != nil {
		return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, fmt.Errorf("%w: %v", ErrInvalidCommentMedia, err)
	}
	if p.mediaEnabled && len(avRefs) > 0 {
		if p.media == nil {
			return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, fmt.Errorf("audio/video checker is not configured")
		}
		decision, err := p.media.CheckExisting(ctx, avRefs)
		if err != nil || decision.Decision != types.MediaModerationDecisionPass {
			return decision, err
		}
	}
	if p.imagesEnabled {
		decision, err := p.checkImages(ctx, imageRefs)
		if err != nil || decision.Decision == types.MediaModerationDecisionReject {
			return decision, err
		}
	}
	return types.CommentMediaDecision{Decision: types.MediaModerationDecisionPass}, nil
}

func (p *commentMediaPolicyImpl) parseAndValidate(content string) ([]types.MediaRef, []types.MediaRef, error) {
	refs, err := utils.ExtractEnabledCommentMediaRefs(
		content, p.publicBucket, p.publicEndpoint, p.maxTotalRefs, p.imagesEnabled, p.mediaEnabled,
	)
	if err != nil {
		return nil, nil, err
	}
	images := make([]types.MediaRef, 0, len(refs))
	av := make([]types.MediaRef, 0, len(refs))
	for _, ref := range refs {
		switch ref.Type {
		case types.MediaTypeImage:
			images = append(images, ref)
		case types.MediaTypeAudio, types.MediaTypeVideo:
			av = append(av, ref)
		default:
			return nil, nil, fmt.Errorf("unsupported comment media type %q", ref.Type)
		}
	}
	if len(images) > p.maxImageRefs {
		return nil, nil, fmt.Errorf("comment references too many images: %d > %d", len(images), p.maxImageRefs)
	}
	if len(av) > p.maxAVRefs {
		return nil, nil, fmt.Errorf("comment references too many audio/video items: %d > %d", len(av), p.maxAVRefs)
	}
	return images, av, nil
}

func (p *commentMediaPolicyImpl) checkImages(ctx context.Context, refs []types.MediaRef) (types.CommentMediaDecision, error) {
	if len(refs) == 0 {
		return types.CommentMediaDecision{Decision: types.MediaModerationDecisionPass}, nil
	}
	if p.images == nil {
		return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, fmt.Errorf("image checker is not configured")
	}
	for _, ref := range refs {
		pass, err := p.images.CheckImageURL(ctx, types.ScenarioImageBaseLineCheck, ref.URL)
		if err != nil {
			return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, fmt.Errorf("check image URL: %w", err)
		}
		if !pass {
			return types.CommentMediaDecision{Decision: types.MediaModerationDecisionReject}, nil
		}
	}
	return types.CommentMediaDecision{Decision: types.MediaModerationDecisionPass}, nil
}
