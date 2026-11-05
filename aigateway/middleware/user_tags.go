// Package middleware provides gin middlewares specific to the aigateway
// service.
package middleware

import (
	"context"
	"log/slog"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

// UserPriorityResolver resolves the upstream channel priority of one user
// identity. restricted is false when the user carries no routing tag (the
// caller is unrestricted). The DB-backed resolver is the production
// implementation; tests inject a fake so the middleware can be exercised
// without a database.
type UserPriorityResolver interface {
	UserPriority(ctx context.Context, userUUID string) (priority int, restricted bool, err error)
}

type dbUserPriorityResolver struct {
	userTags database.UserTagStore
}

// NewDBUserPriorityResolver builds the production resolver over the shared
// database: one join resolves the routing-relevant tags of the user uuid,
// and the tier is derived once (types.UserRoutingPriorityFromTags).
func NewDBUserPriorityResolver() UserPriorityResolver {
	return &dbUserPriorityResolver{userTags: database.NewUserTagStore()}
}

func (r *dbUserPriorityResolver) UserPriority(ctx context.Context, userUUID string) (int, bool, error) {
	tags, err := r.userTags.GetUserRoutingTagsByUUID(ctx, userUUID)
	if err != nil {
		return 0, false, err
	}
	names := make([]string, 0, len(tags))
	for _, tag := range tags {
		names = append(names, tag.Name)
	}
	priority := types.UserRoutingPriorityFromTags(names)
	if priority >= types.UnrestrictedUpstreamPriority {
		return 0, false, nil
	}
	return priority, true, nil
}

// NewUserTagsMiddleware resolves the authenticated user's upstream channel
// priority into the request context. It is attached per route to the
// endpoints that consume it — the inference APIs and the model
// catalogue/detail endpoints — so other authenticated traffic (and other
// services' token verification) pays no extra query.
//
// Identity comes from the global authenticator (JWT, personal token or
// user/org API key; the token owner's uuid is the tag subject). The platform
// system key and anonymous callers carry no priority.
//
// Failure policy: resolution failures are logged and leave the caller
// unrestricted. This is availability-first on purpose — the same database
// outage already fails the model lookup itself, so the window where a
// restricted caller would be over-admitted is the window where inference
// cannot be resolved anyway.
func NewUserTagsMiddleware(resolver UserPriorityResolver) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		userUUID := httpbase.GetCurrentUserUUID(ctx)
		if userUUID == "" || httpbase.GetAuthType(ctx) == httpbase.AuthTypeSystemApiKey {
			ctx.Next()
			return
		}
		priority, restricted, err := resolver.UserPriority(ctx.Request.Context(), userUUID)
		if err != nil {
			slog.WarnContext(ctx.Request.Context(), "failed to resolve user priority, treating caller as unrestricted",
				slog.String("user_uuid", userUUID), slog.Any("error", err))
			ctx.Next()
			return
		}
		if restricted {
			httpbase.SetCurrentUserPriority(ctx, priority)
		}
		ctx.Next()
	}
}
