package component

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

// MediaModerationComponent resolves the moderation state of audio/video
// references and submits new moderation tasks for resources that have no result
// yet. It does not create or edit comments.
type MediaModerationComponent interface {
	// Check resolves the moderation state of every audio/video reference and,
	// for any reference that has no result yet, idempotently submits a new
	// moderation task. It returns a CommentMediaDecision:
	//   - pass:    every reference already has a passing result (no workflow needed).
	//   - pending: at least one reference was just submitted or is still pending;
	//     Items carries the provider task handles the caller needs to poll.
	//   - reject:  a reference was previously rejected.
	//   - error:   the provider or persistence layer could not be reached.
	Check(ctx context.Context, refs []types.MediaRef) (types.CommentMediaDecision, error)
	// CheckExisting resolves cached statuses without submitting provider work.
	CheckExisting(ctx context.Context, refs []types.MediaRef) (types.CommentMediaDecision, error)
	// LinkCommentMedia associates a comment with its media items by inserting
	// idempotent comment_media rows so the visibility logic and the poll
	// workflow can find them by comment.
	LinkCommentMedia(ctx context.Context, commentID int64, items []types.CommentMediaItem) error
}

func (c *mediaModerationComponentImpl) CheckExisting(ctx context.Context, refs []types.MediaRef) (types.CommentMediaDecision, error) {
	if len(refs) == 0 {
		return types.CommentMediaDecision{Decision: types.MediaModerationDecisionPass}, nil
	}
	if c.store == nil {
		return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, fmt.Errorf("media moderation store is not configured")
	}
	keys := make([]string, len(refs))
	for i, ref := range refs {
		key, err := ref.ResourceKey()
		if err != nil {
			return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, fmt.Errorf("compute media resource key: %w", err)
		}
		keys[i] = key
	}
	records, err := c.store.FindByResourceKeys(ctx, keys)
	if err != nil {
		return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, fmt.Errorf("find existing media results: %w", err)
	}
	byKey := make(map[string]database.MediaModeration, len(records))
	for _, record := range records {
		byKey[record.ResourceKey] = record
	}
	decision := types.MediaModerationDecisionPass
	for i, ref := range refs {
		record, found := byKey[keys[i]]
		if !found {
			decision = mergeMediaDecision(decision, types.MediaModerationDecisionPending)
			continue
		}
		if record.MediaType != ref.Type {
			return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, fmt.Errorf(
				"resource %s stored as %q, requested as %q", keys[i], record.MediaType, ref.Type)
		}
		switch record.Status {
		case database.MediaModerationStatusPass:
		case database.MediaModerationStatusReject:
			return types.CommentMediaDecision{Decision: types.MediaModerationDecisionReject}, nil
		case database.MediaModerationStatusPending, database.MediaModerationStatusError:
			decision = mergeMediaDecision(decision, types.MediaModerationDecisionPending)
		default:
			return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, fmt.Errorf(
				"resource %s has unknown status %q", keys[i], record.Status)
		}
	}
	return types.CommentMediaDecision{Decision: decision}, nil
}

type mediaModerationComponentImpl struct {
	store database.MediaModerationStore
	rpc   rpc.ModerationSvcClient
}

func NewMediaModerationComponent(store database.MediaModerationStore, rpc rpc.ModerationSvcClient) MediaModerationComponent {
	return &mediaModerationComponentImpl{store: store, rpc: rpc}
}

func NewMediaModerationComponentFromConfig(cfg *config.Config) (MediaModerationComponent, error) {
	store := database.NewMediaModerationStore()
	client := rpc.NewModerationSvcHttpClient(fmt.Sprintf("%s:%d", cfg.Moderation.Host, cfg.Moderation.Port))
	return NewMediaModerationComponent(store, client), nil
}

func (c *mediaModerationComponentImpl) Check(ctx context.Context, refs []types.MediaRef) (types.CommentMediaDecision, error) {
	if len(refs) == 0 {
		return types.CommentMediaDecision{Decision: types.MediaModerationDecisionPass}, nil
	}
	if c.store == nil || c.rpc == nil {
		return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, fmt.Errorf("media moderation component is not configured")
	}

	keys := make([]string, len(refs))
	for i, ref := range refs {
		if ref.Type != types.MediaTypeAudio && ref.Type != types.MediaTypeVideo {
			return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, fmt.Errorf("unsupported media type %q", ref.Type)
		}
		key, err := ref.ResourceKey()
		if err != nil {
			return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, fmt.Errorf("compute media resource key: %w", err)
		}
		keys[i] = key
	}

	records, err := c.store.FindByResourceKeys(ctx, keys)
	if err != nil {
		return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, fmt.Errorf("find media results: %w", err)
	}
	byKey := make(map[string]database.MediaModeration, len(records))
	for _, record := range records {
		byKey[record.ResourceKey] = record
	}
	// Decide all terminal cached states before creating rows or calling the
	// provider. Otherwise a new resource that appears before a cached reject in
	// refs would be submitted even though the whole comment must be rejected.
	for i, ref := range refs {
		record, exists := byKey[keys[i]]
		if !exists {
			continue
		}
		if record.MediaType != ref.Type {
			return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, fmt.Errorf(
				"resource %s stored as %q, requested as %q", keys[i], record.MediaType, ref.Type)
		}
		switch record.Status {
		case database.MediaModerationStatusPass,
			database.MediaModerationStatusPending,
			database.MediaModerationStatusError:
		case database.MediaModerationStatusReject:
			return types.CommentMediaDecision{Decision: types.MediaModerationDecisionReject}, nil
		default:
			return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, fmt.Errorf(
				"resource %s has unknown status %q", keys[i], record.Status)
		}
	}

	decision := types.MediaModerationDecisionPass
	items := make([]types.CommentMediaItem, 0, len(refs))

	for i, ref := range refs {
		key := keys[i]
		if record, exists := byKey[key]; exists {
			if record.MediaType != ref.Type {
				return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, fmt.Errorf(
					"resource %s stored as %q, requested as %q", key, record.MediaType, ref.Type)
			}
			switch record.Status {
			case database.MediaModerationStatusReject:
				return types.CommentMediaDecision{Decision: types.MediaModerationDecisionReject}, nil
			case database.MediaModerationStatusPass:
				continue
			case database.MediaModerationStatusPending:
				if record.TaskID == "" {
					// A pending row with no task id means a previous submission
					// was never confirmed (e.g. a transport error). Try to
					// claim it for resubmission; if another caller holds the
					// lease, treat it as pending and surface the (empty) task.
					submitted, item, err := c.tryResubmit(ctx, record, ref)
					if err != nil {
						return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, err
					}
					if submitted {
						items = append(items, item)
						decision = mergeMediaDecision(decision, types.MediaModerationDecisionPending)
						continue
					}
					// A workflow snapshots its input, so an empty task ID cannot be
					// recovered after another caller finishes resubmitting. Fail
					// closed and let the client retry once the lease holder has
					// persisted the provider task ID.
					return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, fmt.Errorf(
						"resource %s is being resubmitted and has no task id yet", key)
				}
				// Already submitted and pending. Surface its task handle so the
				// caller can link this comment to the row and poll the existing
				// task; do not resubmit.
				items = append(items, types.CommentMediaItem{
					DataID: record.DataID, TaskID: record.TaskID, MediaType: ref.Type,
				})
				decision = mergeMediaDecision(decision, types.MediaModerationDecisionPending)
				continue
			case database.MediaModerationStatusError:
				// A previous submission errored. Claim it for resubmission so
				// the comment is not permanently blocked by a transient error.
				submitted, item, err := c.tryResubmit(ctx, record, ref)
				if err != nil {
					return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, err
				}
				if submitted {
					items = append(items, item)
					decision = mergeMediaDecision(decision, types.MediaModerationDecisionPending)
					continue
				}
				// Not eligible yet (lease held) or another caller is
				// resubmitting. Fail closed for this request so the comment is
				// not created in a state the workflow cannot resolve.
				return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, fmt.Errorf(
					"resource %s previously errored and is not yet eligible for resubmission", key)
			default:
				return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, fmt.Errorf(
					"resource %s has unknown status %q", key, record.Status)
			}
		}

		// No record yet: create an idempotent pending row and submit it.
		dataID := uuid.NewString()
		seed := strings.ReplaceAll(uuid.NewString(), "-", "")
		leaseAt := time.Now().UTC()
		created, isNew, err := c.store.CreateOrGet(ctx, database.MediaModeration{
			ResourceKey:   key,
			DataID:        dataID,
			Seed:          seed,
			MediaType:     ref.Type,
			Status:        database.MediaModerationStatusPending,
			LastAttemptAt: &leaseAt,
		})
		if err != nil {
			return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, fmt.Errorf("create media result: %w", err)
		}
		if !isNew {
			// Another caller raced ahead and created the row. Re-evaluate it on
			// the next loop iteration is not possible (we already advanced), so
			// honor its state directly here.
			if created.MediaType != ref.Type {
				return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, fmt.Errorf(
					"resource %s stored as %q, requested as %q", key, created.MediaType, ref.Type)
			}
			switch created.Status {
			case database.MediaModerationStatusPass:
				continue
			case database.MediaModerationStatusReject:
				return types.CommentMediaDecision{Decision: types.MediaModerationDecisionReject}, nil
			case database.MediaModerationStatusPending, database.MediaModerationStatusError:
				// Treat like an existing row: resubmit if eligible. An empty task ID
				// cannot be handed to a workflow when another caller owns the lease.
				submitted, item, err := c.tryResubmit(ctx, *created, ref)
				if err != nil {
					return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, err
				}
				if submitted {
					items = append(items, item)
					decision = mergeMediaDecision(decision, types.MediaModerationDecisionPending)
					continue
				}
				if created.TaskID == "" {
					return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, fmt.Errorf(
						"resource %s is being submitted and has no task id yet", key)
				}
				items = append(items, types.CommentMediaItem{
					DataID: created.DataID, TaskID: created.TaskID, MediaType: ref.Type,
				})
				decision = mergeMediaDecision(decision, types.MediaModerationDecisionPending)
				continue
			default:
				return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, fmt.Errorf(
					"resource %s has unknown status %q", key, created.Status)
			}
		}

		item, err := c.submitAndMark(ctx, *created, ref, leaseAt)
		if err != nil {
			return types.CommentMediaDecision{Decision: types.MediaModerationDecisionError}, err
		}
		items = append(items, item)
		decision = mergeMediaDecision(decision, types.MediaModerationDecisionPending)
	}

	return types.CommentMediaDecision{Decision: decision, Items: items}, nil
}

// resubmitLease bounds how long a claimed-for-resubmission row is considered
// in-flight before another caller may reclaim it.
const resubmitLease = time.Minute

// tryResubmit claims an error or unsubmitted-pending row for resubmission and,
// on success, submits it and marks it submitted. It returns submitted=true with
// the resulting CommentMediaItem when this caller performed the resubmission.
// When the row is not eligible (lease held by another caller), it returns
// submitted=false with no error so the caller can surface the row as pending.
func (c *mediaModerationComponentImpl) tryResubmit(ctx context.Context, record database.MediaModeration, ref types.MediaRef) (bool, types.CommentMediaItem, error) {
	leaseAt := time.Now().UTC()
	claimed, err := c.store.ClaimForResubmit(ctx, record.DataID, leaseAt.Add(-resubmitLease), leaseAt)
	if err != nil {
		return false, types.CommentMediaItem{}, fmt.Errorf("claim media moderation for resubmit: %w", err)
	}
	if !claimed {
		return false, types.CommentMediaItem{}, nil
	}
	// Claimed: submit and mark. The row's data_id/seed are reused so the
	// provider callback (if any) still resolves to the same row.
	item, err := c.submitAndMark(ctx, record, ref, leaseAt)
	if err != nil {
		return false, types.CommentMediaItem{}, err
	}
	return true, item, nil
}

// submitAndMark submits one media resource to the provider and persists the
// returned task id. On a transport error it leaves the row pending with an
// empty task id (eligible for a later ClaimForResubmit) and returns the error
// so the caller fails closed.
func (c *mediaModerationComponentImpl) submitAndMark(ctx context.Context, record database.MediaModeration, ref types.MediaRef, leaseAt time.Time) (types.CommentMediaItem, error) {
	submission, submitErr := c.rpc.SubmitMediaModeration(ctx, types.MediaModerationRequest{
		URL: ref.URL, Type: ref.Type, DataID: record.DataID, Seed: record.Seed,
	})
	if submitErr != nil {
		slog.ErrorContext(ctx, "submit media moderation failed", slog.String("data_id", record.DataID), slog.Any("error", submitErr))
		return types.CommentMediaItem{}, fmt.Errorf("submit media moderation: %w", submitErr)
	}
	if submission == nil || submission.DataID != record.DataID || submission.Seed != record.Seed || submission.TaskID == "" {
		return types.CommentMediaItem{}, fmt.Errorf("media moderation provider returned a mismatched submission identity")
	}
	if err := c.store.MarkSubmitted(ctx, record.DataID, submission.TaskID, leaseAt); err != nil {
		return types.CommentMediaItem{}, fmt.Errorf("mark media moderation submitted: %w", err)
	}
	return types.CommentMediaItem{
		DataID: record.DataID, TaskID: submission.TaskID, MediaType: ref.Type,
	}, nil
}

func (c *mediaModerationComponentImpl) LinkCommentMedia(ctx context.Context, commentID int64, items []types.CommentMediaItem) error {
	if c.store == nil {
		return fmt.Errorf("media moderation store is not configured")
	}
	if len(items) == 0 {
		return nil
	}
	return c.store.LinkCommentMedia(ctx, commentID, items)
}

func mergeMediaDecision(current, next types.MediaModerationDecision) types.MediaModerationDecision {
	priority := map[types.MediaModerationDecision]int{
		types.MediaModerationDecisionPass: 0, types.MediaModerationDecisionPending: 1,
		types.MediaModerationDecisionError: 2, types.MediaModerationDecisionReject: 3,
	}
	if priority[next] > priority[current] {
		return next
	}
	return current
}
