package database_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/types/enum"
)

func TestUserTagStoreImpl_ResetUserTags(t *testing.T) {
	ctx := context.Background()
	testDB := tests.InitTestDB()
	userTagStore := database.NewUserTagStoreWithDB(testDB)

	userId := int64(1)
	tagIDs := []int64{1, 2, 3}

	t.Run("ResetUserTags", func(t *testing.T) {
		err := userTagStore.ResetUserTags(ctx, userId, tagIDs)
		if err != nil {
			t.Errorf("ResetUserTags failed, err: %v", err)
		}
	})

}

func TestUserTagStoreImpl_GetUserRoutingTagsByUUID(t *testing.T) {
	ctx := context.Background()
	testDB := tests.InitTestDB()
	defer testDB.Close()
	userTagStore := database.NewUserTagStoreWithDB(testDB)

	// Per-run suffix keeps the test data unique on reused test databases.
	suffix := time.Now().UnixNano()
	user := database.User{
		UUID:     fmt.Sprintf("routing-tags-user-%d", suffix),
		GitID:    suffix,
		Username: fmt.Sprintf("routing-tags-user-%d", suffix),
		NickName: "Routing Tags User",
		Email:    fmt.Sprintf("routing-tags-%d@example.com", suffix),
	}
	_, err := testDB.Core.NewInsert().Model(&user).Exec(ctx)
	require.Nil(t, err)

	// ensureTag resolves a tag by its unique key, inserting it only when it
	// does not exist yet: the user-scope seed migration already ships
	// low_priority, so a plain insert would violate idx_tags_name_category_scope.
	ensureTag := func(def database.Tag) *database.Tag {
		tag := def
		_, insertErr := testDB.Core.NewInsert().Model(&tag).
			On("CONFLICT (name, category, scope) DO NOTHING").
			Exec(ctx)
		require.Nil(t, insertErr)
		if tag.ID == 0 {
			tag = database.Tag{}
			selectErr := testDB.Core.NewSelect().Model(&tag).
				Where("name = ? AND category = ? AND scope = ?", def.Name, def.Category, def.Scope).
				Limit(1).
				Scan(ctx)
			require.Nil(t, selectErr)
		}
		return &tag
	}

	// One routing tag of each kind plus unrelated tags that must not be
	// returned: a topic tag, and a model-scope tag that happens to use the
	// priority:<n> naming (only user-scope tags may affect routing).
	tags := []database.Tag{
		{Name: "Automotive", Category: "topic", Group: "topic", Scope: "model"},
		{Name: types.UserTagLowPriority, Category: "user", Group: "user", Scope: "user"},
		{Name: types.UserTagPriorityPrefix + "50", Category: "user", Group: "user", Scope: "user"},
		{Name: types.UserTagPriorityPrefix + "99", Category: "topic", Group: "topic", Scope: "model"},
	}
	for i := range tags {
		tag := ensureTag(tags[i])
		_, err = testDB.Core.NewInsert().Model(&database.UserTag{UserID: user.ID, TagID: tag.ID}).Exec(ctx)
		require.Nil(t, err)
	}

	got, err := userTagStore.GetUserRoutingTagsByUUID(ctx, user.UUID)
	require.Nil(t, err)
	names := make([]string, 0, len(got))
	for _, tag := range got {
		names = append(names, tag.Name)
	}
	require.ElementsMatch(t, []string{types.UserTagLowPriority, "priority:50"}, names)
	require.Equal(t, 50, types.UserRoutingPriorityFromTags(names))

	// A user without routing tags yields no rows, and so does an unknown
	// uuid.
	plainUser := database.User{
		UUID:     fmt.Sprintf("plain-tags-user-%d", suffix),
		GitID:    suffix + 1,
		Username: fmt.Sprintf("plain-tags-user-%d", suffix),
		NickName: "Plain User",
		Email:    fmt.Sprintf("plain-tags-%d@example.com", suffix),
	}
	_, err = testDB.Core.NewInsert().Model(&plainUser).Exec(ctx)
	require.Nil(t, err)
	empty, err := userTagStore.GetUserRoutingTagsByUUID(ctx, plainUser.UUID)
	require.Nil(t, err)
	require.Empty(t, empty)
	missing, err := userTagStore.GetUserRoutingTagsByUUID(ctx, "no-such-uuid")
	require.Nil(t, err)
	require.Empty(t, missing)
}

// TestUserTagStoreImpl_ResetUserTagsPreservesRoutingTags covers the
// self-service tag reset used by the personal tag endpoint and the profile
// update path: operator-managed routing tags must survive it, so a restricted
// user cannot lift their own upstream isolation by clearing or overriding
// their tags.
func TestUserTagStoreImpl_ResetUserTagsPreservesRoutingTags(t *testing.T) {
	ctx := context.Background()
	testDB := tests.InitTestDB()
	defer testDB.Close()
	userTagStore := database.NewUserTagStoreWithDB(testDB)

	suffix := time.Now().UnixNano()
	user := database.User{
		UUID:     fmt.Sprintf("reset-routing-user-%d", suffix),
		GitID:    suffix,
		Username: fmt.Sprintf("reset-routing-user-%d", suffix),
		NickName: "Reset Routing User",
		Email:    fmt.Sprintf("reset-routing-%d@example.com", suffix),
	}
	_, err := testDB.Core.NewInsert().Model(&user).Exec(ctx)
	require.Nil(t, err)

	ensureTag := func(def database.Tag) *database.Tag {
		tag := def
		_, insertErr := testDB.Core.NewInsert().Model(&tag).
			On("CONFLICT (name, category, scope) DO NOTHING").
			Exec(ctx)
		require.Nil(t, insertErr)
		if tag.ID == 0 {
			tag = database.Tag{}
			selectErr := testDB.Core.NewSelect().Model(&tag).
				Where("name = ? AND category = ? AND scope = ?", def.Name, def.Category, def.Scope).
				Limit(1).
				Scan(ctx)
			require.Nil(t, selectErr)
		}
		return &tag
	}

	lowPriority := ensureTag(database.Tag{Name: types.UserTagLowPriority, Category: "user", Group: "user", Scope: "user"})
	tier := ensureTag(database.Tag{Name: types.UserTagPriorityPrefix + "50", Category: "user", Group: "user", Scope: "user"})
	profile := ensureTag(database.Tag{Name: fmt.Sprintf("profile-%d", suffix), Category: "user", Group: "user", Scope: "user"})
	for _, tag := range []*database.Tag{lowPriority, tier, profile} {
		_, err = testDB.Core.NewInsert().Model(&database.UserTag{UserID: user.ID, TagID: tag.ID}).Exec(ctx)
		require.Nil(t, err)
	}

	names := func() []string {
		tags, err := userTagStore.GetUserTags(ctx, user.ID)
		require.Nil(t, err)
		out := make([]string, 0, len(tags))
		for _, tag := range tags {
			out = append(out, tag.Name)
		}
		return out
	}
	want := []string{types.UserTagLowPriority, types.UserTagPriorityPrefix + "50", profile.Name}
	operatorReset := func(ids []int64) database.UserTagOperatorResetRequest {
		return database.UserTagOperatorResetRequest{
			UserID: user.ID, TagIDs: ids, OperatorID: user.UUID, UserUUID: user.UUID, UserName: user.Username,
		}
	}

	// Resetting with only the profile tag keeps both operator routing tags.
	require.Nil(t, userTagStore.ResetUserTags(ctx, user.ID, []int64{profile.ID}))
	require.ElementsMatch(t, want, names())

	// Granting a routing tag through the same entry point is ignored, and the
	// existing routing tags are neither duplicated nor removed.
	require.Nil(t, userTagStore.ResetUserTags(ctx, user.ID, []int64{profile.ID, tier.ID, lowPriority.ID}))
	require.ElementsMatch(t, want, names())

	// A same-named tag from another scope is not a routing entitlement: the
	// write-side guard matches scope='user' (like the read side), so the
	// self-service reset clears it as an ordinary label.
	modelScoped := ensureTag(database.Tag{Name: types.UserTagPriorityPrefix + "99", Category: "topic", Group: "topic", Scope: "model"})
	_, err = testDB.Core.NewInsert().Model(&database.UserTag{UserID: user.ID, TagID: modelScoped.ID}).Exec(ctx)
	require.Nil(t, err)

	// An empty reset clears self-service tags only.
	require.Nil(t, userTagStore.ResetUserTags(ctx, user.ID, nil))
	require.ElementsMatch(t, want[:2], names())

	// The operator path replaces the USER-SCOPE subset as requested (routing
	// tags included) but preserves tags of other scopes: the admin surface
	// never shows them, so saving must not delete them. The returned set is
	// the exact user-scope state that was replaced (read inside the tx).
	_, err = testDB.Core.NewInsert().Model(&database.UserTag{UserID: user.ID, TagID: modelScoped.ID}).Exec(ctx)
	require.Nil(t, err)
	before, err := userTagStore.ResetUserTagsAsOperator(ctx, operatorReset([]int64{tier.ID}))
	require.Nil(t, err)
	beforeNames := make([]string, 0, len(before))
	for _, tag := range before {
		beforeNames = append(beforeNames, tag.Name)
	}
	require.ElementsMatch(t, want[:2], beforeNames)
	require.ElementsMatch(t, []string{tier.Name, modelScoped.Name}, names())
	_, err = userTagStore.ResetUserTagsAsOperator(ctx, operatorReset([]int64{}))
	require.Nil(t, err)
	require.ElementsMatch(t, []string{modelScoped.Name}, names())

	var logs []database.AuditLog
	err = testDB.Core.NewSelect().Model(&logs).
		Where("table_name = ? AND resource_id = ?", "user_tags", user.UUID).
		Order("id ASC").Scan(ctx)
	require.Nil(t, err)
	require.Len(t, logs, 2)
	var firstBefore struct {
		TagIDs   []int64  `json:"tag_ids"`
		TagNames []string `json:"tag_names"`
	}
	var firstAfter struct {
		TagIDs []int64 `json:"tag_ids"`
	}
	var secondBefore struct {
		TagIDs   []int64  `json:"tag_ids"`
		TagNames []string `json:"tag_names"`
	}
	var secondAfter struct {
		TagIDs []int64 `json:"tag_ids"`
	}
	require.NoError(t, json.Unmarshal(logs[0].Before, &firstBefore))
	require.NoError(t, json.Unmarshal(logs[0].After, &firstAfter))
	require.NoError(t, json.Unmarshal(logs[1].Before, &secondBefore))
	require.NoError(t, json.Unmarshal(logs[1].After, &secondAfter))
	require.ElementsMatch(t, []int64{lowPriority.ID, tier.ID}, firstBefore.TagIDs)
	require.ElementsMatch(t, []string{lowPriority.Name, tier.Name}, firstBefore.TagNames)
	require.Equal(t, []int64{tier.ID}, firstAfter.TagIDs)
	require.Equal(t, []int64{tier.ID}, secondBefore.TagIDs)
	require.Equal(t, []string{tier.Name}, secondBefore.TagNames)
	require.NotNil(t, secondAfter.TagIDs)
	require.Empty(t, secondAfter.TagIDs)
}

// TestUserTagStoreImpl_OperatorResetConcurrent verifies the per-user advisory
// lock serializes concurrent replacements: the final set must be exactly one
// of the two requests, never their union.
func TestUserTagStoreImpl_OperatorResetConcurrent(t *testing.T) {
	ctx := context.Background()
	// The txdb harness (InitTestDB) multiplexes all queries through one
	// session, so cross-session locking cannot be exercised; this test needs
	// real connections.
	testDB := tests.InitTransactionTestDB()
	defer testDB.Close()
	userTagStore := database.NewUserTagStoreWithDB(testDB)

	suffix := time.Now().UnixNano()
	user := database.User{
		UUID:     fmt.Sprintf("concurrent-user-%d", suffix),
		GitID:    suffix,
		Username: fmt.Sprintf("concurrent-user-%d", suffix),
		NickName: "Concurrent User",
		Email:    fmt.Sprintf("concurrent-%d@example.com", suffix),
	}
	_, err := testDB.Core.NewInsert().Model(&user).Exec(ctx)
	require.Nil(t, err)

	ensureTag := func(def database.Tag) *database.Tag {
		tag := def
		_, insertErr := testDB.Core.NewInsert().Model(&tag).
			On("CONFLICT (name, category, scope) DO NOTHING").
			Exec(ctx)
		require.Nil(t, insertErr)
		if tag.ID == 0 {
			tag = database.Tag{}
			require.Nil(t, testDB.Core.NewSelect().Model(&tag).
				Where("name = ? AND category = ? AND scope = ?", def.Name, def.Category, def.Scope).
				Limit(1).Scan(ctx))
		}
		return &tag
	}
	low := ensureTag(database.Tag{Name: types.UserTagLowPriority, Category: "user", Group: "user", Scope: "user"})
	high := ensureTag(database.Tag{Name: types.UserTagPriorityPrefix + "100", Category: "user", Group: "user", Scope: "user"})

	for i := 0; i < 5; i++ {
		var wg sync.WaitGroup
		errs := make(chan error, 2)
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, err := userTagStore.ResetUserTagsAsOperator(ctx, database.UserTagOperatorResetRequest{
				UserID: user.ID, TagIDs: []int64{low.ID}, OperatorID: user.UUID, UserUUID: user.UUID, UserName: user.Username,
			})
			errs <- err
		}()
		go func() {
			defer wg.Done()
			_, err := userTagStore.ResetUserTagsAsOperator(ctx, database.UserTagOperatorResetRequest{
				UserID: user.ID, TagIDs: []int64{high.ID}, OperatorID: user.UUID, UserUUID: user.UUID, UserName: user.Username,
			})
			errs <- err
		}()
		wg.Wait()
		close(errs)
		for err := range errs {
			require.Nil(t, err)
		}

		tags, err := userTagStore.GetUserTags(ctx, user.ID)
		require.Nil(t, err)
		require.Len(t, tags, 1, "concurrent replacements must not merge into a union")
		require.Contains(t, []string{low.Name, high.Name}, tags[0].Name)
	}
}

func TestUserTagStoreImpl_OperatorResetRollsBackWhenAuditInsertFails(t *testing.T) {
	ctx := context.Background()
	cfg, err := config.LoadConfig()
	require.NoError(t, err)
	cfg.AuditLog.Enabled = true
	testDB := tests.InitTransactionTestDBWithHooks()
	defer testDB.Close()
	userTagStore := database.NewUserTagStoreWithDB(testDB)

	suffix := time.Now().UnixNano()
	user := database.User{
		UUID:     fmt.Sprintf("audit-failure-user-%d", suffix),
		GitID:    suffix,
		Username: fmt.Sprintf("audit-failure-user-%d", suffix),
		NickName: "Audit Failure User",
		Email:    fmt.Sprintf("audit-failure-%d@example.com", suffix),
	}
	_, err = testDB.Core.NewInsert().Model(&user).Exec(ctx)
	require.NoError(t, err)
	beforeTag := database.Tag{Name: fmt.Sprintf("before-%d", suffix), Category: "user", Group: "user", Scope: "user"}
	afterTag := database.Tag{Name: fmt.Sprintf("after-%d", suffix), Category: "user", Group: "user", Scope: "user"}
	for _, tag := range []*database.Tag{&beforeTag, &afterTag} {
		_, err = testDB.Core.NewInsert().Model(tag).Exec(ctx)
		require.NoError(t, err)
	}
	_, err = testDB.Core.NewInsert().Model(&database.UserTag{UserID: user.ID, TagID: beforeTag.ID}).Exec(ctx)
	require.NoError(t, err)

	// A distinct operator (not the target user) proves the audit row names
	// the acting admin: operator_id/user_name carry the operator, resource_id
	// the target. This mirrors the P1 review ask that Alice editing Bob must
	// attribute the change to Alice.
	operator := database.User{
		UUID:     fmt.Sprintf("audit-operator-%d", suffix),
		GitID:    suffix + 1,
		Username: fmt.Sprintf("audit-operator-%d", suffix),
		NickName: "Audit Operator",
		Email:    fmt.Sprintf("audit-operator-%d@example.com", suffix),
	}
	_, err = testDB.Core.NewInsert().Model(&operator).Exec(ctx)
	require.NoError(t, err)

	_, err = testDB.BunDB.NewRaw("ALTER TABLE audit_logs RENAME TO audit_logs_test_unavailable").Exec(ctx)
	require.NoError(t, err)
	restored := false
	restoreAuditLogs := func() error {
		if restored {
			return nil
		}
		_, restoreErr := testDB.BunDB.NewRaw("ALTER TABLE audit_logs_test_unavailable RENAME TO audit_logs").Exec(context.Background())
		if restoreErr == nil {
			restored = true
		}
		return restoreErr
	}
	defer func() { _ = restoreAuditLogs() }()

	_, err = userTagStore.ResetUserTagsAsOperator(ctx, database.UserTagOperatorResetRequest{
		UserID: user.ID, TagIDs: []int64{afterTag.ID}, OperatorID: user.UUID, UserUUID: user.UUID, UserName: user.Username,
	})
	require.ErrorContains(t, err, "insert user tag audit log")

	after, err := userTagStore.GetUserTags(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, after, 1)
	require.Equal(t, beforeTag.ID, after[0].ID, "the tag replacement must roll back with the failed audit insert")
	require.NoError(t, restoreAuditLogs())

	_, err = userTagStore.ResetUserTagsAsOperator(ctx, database.UserTagOperatorResetRequest{
		UserID: user.ID, TagIDs: []int64{afterTag.ID}, OperatorID: operator.UUID, UserUUID: user.UUID, UserName: operator.Username,
	})
	require.NoError(t, err)
	var logs []database.AuditLog
	require.NoError(t, testDB.Core.NewSelect().Model(&logs).
		Where("table_name = ? AND resource_id = ?", "user_tags", user.UUID).
		Scan(ctx))
	require.Len(t, logs, 1, "the aggregate audit event should replace per-row hook entries")
	require.Equal(t, enum.AuditActionUpdate, logs[0].Action)
	require.Equal(t, operator.UUID, logs[0].OperatorID, "audit operator must be the acting admin, not the target user")
	require.Equal(t, operator.Username, logs[0].UserName, "audit user_name must be the acting admin, not the target user")
	require.Equal(t, user.UUID, logs[0].ResourceID, "the audit resource is the target user")
	require.JSONEq(t, `{"tag_ids":[`+strconv.FormatInt(afterTag.ID, 10)+`]}`, string(logs[0].After))
}
