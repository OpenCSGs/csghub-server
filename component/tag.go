package component

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component/tagparser"
)

type TagComponent interface {
	AllTags(ctx context.Context, filter *types.TagFilter) ([]*types.RepoTag, error)
	ClearMetaTags(ctx context.Context, repoType types.RepositoryType, namespace, name string) error
	UpdateMetaTags(ctx context.Context, tagScope types.TagScope, namespace, name, content string) ([]*database.RepositoryTag, error)
	UpdateLibraryTags(ctx context.Context, tagScope types.TagScope, namespace, name, oldFilePath, newFilePath string) error
	UpdateRepoTagsByCategory(ctx context.Context, tagScope types.TagScope, repoID int64, category string, tagNames []string) error
	CreateTag(ctx context.Context, username string, req types.CreateTag) (*database.Tag, error)
	GetTagByID(ctx context.Context, username string, id int64) (*database.Tag, error)
	UpdateTag(ctx context.Context, username string, id int64, req types.UpdateTag) (*database.Tag, error)
	DeleteTag(ctx context.Context, username string, id int64) error
	AllCategories(ctx context.Context) ([]database.TagCategory, error)
	CreateCategory(ctx context.Context, username string, req types.CreateCategory) (*database.TagCategory, error)
	UpdateCategory(ctx context.Context, username string, req types.UpdateCategory, id int64) (*database.TagCategory, error)
	DeleteCategory(ctx context.Context, username string, id int64) error
}

func NewTagComponent(config *config.Config) (TagComponent, error) {
	tc := &tagComponentImpl{}
	tc.tagStore = database.NewTagStore()
	tc.repoStore = database.NewRepoStore()
	if config.SensitiveCheck.Enable {
		tc.sensitiveChecker = rpc.NewModerationSvcHttpClient(fmt.Sprintf("%s:%d", config.Moderation.Host, config.Moderation.Port))
	}
	tc.userStore = database.NewUserStore()
	return tc, nil
}

type tagComponentImpl struct {
	tagStore         database.TagStore
	repoStore        database.RepoStore
	sensitiveChecker rpc.ModerationSvcClient
	userStore        database.UserStore
}

func (tc *tagComponentImpl) AllTags(ctx context.Context, filter *types.TagFilter) ([]*types.RepoTag, error) {
	tags, err := tc.tagStore.AllTags(ctx, filter)
	if err != nil {
		return nil, err
	}
	resTags := make([]*types.RepoTag, len(tags))
	for i, tag := range tags {
		resTags[i] = &types.RepoTag{
			ID:        tag.ID,
			Name:      tag.Name,
			Category:  tag.Category,
			Group:     tag.Group,
			Scope:     tag.Scope,
			BuiltIn:   tag.BuiltIn,
			ShowName:  tag.I18nKey,
			I18nKey:   tag.I18nKey,
			CreatedAt: tag.CreatedAt,
			UpdatedAt: tag.UpdatedAt,
		}
	}
	return resTags, nil
}

func (c *tagComponentImpl) ClearMetaTags(ctx context.Context, repoType types.RepositoryType, namespace, name string) error {

	_, err := c.tagStore.SetMetaTags(ctx, repoType, namespace, name, nil)
	return err
}

func (c *tagComponentImpl) UpdateMetaTags(ctx context.Context, tagScope types.TagScope, namespace, name, content string) ([]*database.RepositoryTag, error) {
	var (
		tp       tagparser.TagProcessor
		repoType types.RepositoryType
	)
	// TODO:load from cache

	if tagScope == types.DatasetTagScope {
		tp = tagparser.NewDatasetTagProcessor(c.tagStore)
		repoType = types.DatasetRepo
	} else if tagScope == types.ModelTagScope {
		tp = tagparser.NewModelTagProcessor(c.tagStore)
		repoType = types.ModelRepo
	} else if tagScope == types.PromptTagScope {
		tp = tagparser.NewPromptTagProcessor(c.tagStore)
		repoType = types.PromptRepo
	} else if tagScope == types.MCPTagScope {
		tp = tagparser.NewMCPTagProcessor(c.tagStore)
		repoType = types.MCPServerRepo
	} else if tagScope == types.CodeTagScope {
		tp = tagparser.NewCodeTagProcessor(c.tagStore)
		repoType = types.CodeRepo
	} else if tagScope == types.SpaceTagScope {
		tp = tagparser.NewSpaceTagProcessor(c.tagStore)
		repoType = types.SpaceRepo
	} else {
		return nil, nil
	}
	tagsMatched, tagToCreate, err := tp.ProcessReadme(ctx, content)
	if err != nil {
		slog.Error("Failed to process tags", slog.Any("error", err))
		return nil, fmt.Errorf("failed to process tags, cause: %w", err)
	}

	if c.sensitiveChecker != nil {
		// TODO:do tag name sensitive checking in batch
		// remove sensitive tags by checking tag name of tag to create
		tagToCreate = slices.DeleteFunc(tagToCreate, func(t *database.Tag) bool {
			result, err := c.sensitiveChecker.PassTextCheck(ctx, string(sensitive.ScenarioNicknameDetection), t.Name)
			if err != nil {
				slog.Error("Failed to check tag name sensitivity", slog.String("tag_name", t.Name), slog.Any("error", err))
				return true
			}
			return result.IsSensitive
		})
	}

	err = c.tagStore.SaveTags(ctx, tagToCreate)
	if err != nil {
		slog.Error("Failed to save tags", slog.Any("error", err))
		return nil, fmt.Errorf("failed to save tags, cause: %w", err)
	}

	repo, err := c.repoStore.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		slog.Error("failed to find repo", slog.Any("error", err))
		return nil, fmt.Errorf("failed to find repo, cause: %w", err)
	}

	metaTags := append(tagsMatched, tagToCreate...)
	var repoTags []*database.RepositoryTag
	repoTags, err = c.tagStore.SetMetaTags(ctx, repoType, namespace, name, metaTags)
	if err != nil {
		slog.Error("failed to set dataset's tags", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("error", err))
		return nil, fmt.Errorf("failed to set dataset's tags, cause: %w", err)
	}

	err = c.repoStore.UpdateLicenseByTag(ctx, repo.ID)
	if err != nil {
		slog.Error("failed to update repo license tags", slog.Any("error", err))
	}

	return repoTags, nil
}

func (c *tagComponentImpl) UpdateLibraryTags(ctx context.Context, tagScope types.TagScope, namespace, name, oldFilePath, newFilePath string) error {
	oldLibTagName := tagparser.LibraryTag(oldFilePath)
	newLibTagName := tagparser.LibraryTag(newFilePath)
	// TODO:load from cache
	var (
		allTags  []*database.Tag
		err      error
		repoType types.RepositoryType
	)
	if tagScope == types.DatasetTagScope {
		allTags, err = c.tagStore.AllDatasetTags(ctx)
		repoType = types.DatasetRepo
	} else if tagScope == types.ModelTagScope {
		allTags, err = c.tagStore.AllModelTags(ctx)
		repoType = types.ModelRepo
	} else if tagScope == types.PromptTagScope {
		allTags, err = c.tagStore.AllPromptTags(ctx)
		repoType = types.PromptRepo
	} else {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get all tags, error: %w", err)
	}
	var oldLibTag, newLibTag *database.Tag
	for _, t := range allTags {
		if t.Category != "framework" {
			continue
		}
		if t.Name == newLibTagName {
			newLibTag = t
		}
		if t.Name == oldLibTagName {
			oldLibTag = t
		}
	}
	err = c.tagStore.SetLibraryTag(ctx, repoType, namespace, name, newLibTag, oldLibTag)
	if err != nil {
		slog.Error("failed to set %s's tags", string(repoType), slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("failed to set Library tags, cause: %w", err)
	}
	return nil
}

func (c *tagComponentImpl) UpdateRepoTagsByCategory(ctx context.Context, tagScope types.TagScope, repoID int64, category string, tagNames []string) error {
	filter := &types.TagFilter{
		Scopes:     []types.TagScope{tagScope},
		Categories: []string{category},
	}
	allTags, err := c.tagStore.AllTags(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to get all tags of scope `%s`, error: %w", tagScope, err)
	}

	if len(allTags) == 0 {
		return fmt.Errorf("no tags found for scope `%s` and category `%s`", tagScope, category)
	}

	var tagIDs []int64
	for _, tagName := range tagNames {
		for _, t := range allTags {
			if t.Name == tagName {
				tagIDs = append(tagIDs, t.ID)
			}
		}
	}

	var oldTagIDs []int64
	oldTagIDs, err = c.repoStore.TagIDs(ctx, repoID, category)
	if err != nil {
		return fmt.Errorf("failed to get old tag ids, error: %w", err)
	}
	return c.tagStore.UpsertRepoTags(ctx, repoID, oldTagIDs, tagIDs)
}

func (c *tagComponentImpl) CreateTag(ctx context.Context, username string, req types.CreateTag) (*database.Tag, error) {
	user, err := c.userStore.FindByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("failed to get user, error: %w", err)
	}
	if !user.CanAdmin() {
		return nil, fmt.Errorf("user %s do not allowed create tag", username)
	}

	if c.sensitiveChecker != nil {
		result, err := c.sensitiveChecker.PassTextCheck(ctx, string(sensitive.ScenarioNicknameDetection), req.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to check tag name sensitivity, error: %w", err)
		}
		if result.IsSensitive {
			return nil, fmt.Errorf("tag name contains sensitive words")
		}
	}

	newTag := database.Tag{
		Name:     req.Name,
		Category: req.Category,
		Group:    req.Group,
		Scope:    types.TagScope(req.Scope),
		BuiltIn:  req.BuiltIn,
		ShowName: req.ShowName,
		I18nKey:  req.I18nKey,
	}

	tag, err := c.tagStore.CreateTag(ctx, newTag)
	if err != nil {
		return nil, fmt.Errorf("failed to create tag, error: %w", err)
	}
	return tag, nil
}

func (c *tagComponentImpl) GetTagByID(ctx context.Context, username string, id int64) (*database.Tag, error) {
	user, err := c.userStore.FindByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("failed to get user, error: %w", err)
	}
	if !user.CanAdmin() {
		return nil, fmt.Errorf("user %s do not allowed create tag", username)
	}
	tag, err := c.tagStore.FindTagByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get tag id %d, error: %w", id, err)
	}
	return tag, nil
}

func (c *tagComponentImpl) UpdateTag(ctx context.Context, username string, id int64, req types.UpdateTag) (*database.Tag, error) {
	user, err := c.userStore.FindByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("failed to get user, error: %w", err)
	}
	if !user.CanAdmin() {
		return nil, fmt.Errorf("user %s do not allowed create tag", username)
	}

	if c.sensitiveChecker != nil {
		result, err := c.sensitiveChecker.PassTextCheck(ctx, string(sensitive.ScenarioNicknameDetection), req.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to check tag name sensitivity, error: %w", err)
		}
		if result.IsSensitive {
			return nil, fmt.Errorf("tag name contains sensitive words")
		}
	}

	tag := &database.Tag{
		ID:       id,
		Category: req.Category,
		Name:     req.Name,
		Group:    req.Group,
		Scope:    types.TagScope(req.Scope),
		BuiltIn:  req.BuiltIn,
		ShowName: req.ShowName,
		I18nKey:  req.I18nKey,
	}
	newTag, err := c.tagStore.UpdateTagByID(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("failed to update tag id %d, error: %w", id, err)
	}
	return newTag, nil
}

func (c *tagComponentImpl) DeleteTag(ctx context.Context, username string, id int64) error {
	user, err := c.userStore.FindByUsername(ctx, username)
	if err != nil {
		return fmt.Errorf("failed to get user, error: %w", err)
	}
	if !user.CanAdmin() {
		return fmt.Errorf("user %s do not allowed create tag", username)
	}
	err = c.tagStore.DeleteTagByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete tag id %d, error: %w", id, err)
	}
	return nil
}

func (c *tagComponentImpl) AllCategories(ctx context.Context) ([]database.TagCategory, error) {
	return c.tagStore.AllCategories(ctx, types.TagScope(""))
}

func (c *tagComponentImpl) CreateCategory(ctx context.Context, username string, req types.CreateCategory) (*database.TagCategory, error) {
	user, err := c.userStore.FindByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("failed to get user, error: %w", err)
	}
	if !user.CanAdmin() {
		return nil, errorx.ErrForbidden
	}

	newCategory := database.TagCategory{
		Name:     req.Name,
		ShowName: req.ShowName,
		Scope:    types.TagScope(req.Scope),
		Enabled:  req.Enabled,
	}

	category, err := c.tagStore.CreateCategory(ctx, newCategory)
	if err != nil {
		return nil, fmt.Errorf("failed to create category, error: %w", err)
	}

	return category, nil
}

func (c *tagComponentImpl) UpdateCategory(ctx context.Context, username string, req types.UpdateCategory, id int64) (*database.TagCategory, error) {
	user, err := c.userStore.FindByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("failed to get user, error: %w", err)
	}
	if !user.CanAdmin() {
		return nil, errorx.ErrForbidden
	}

	newCategory := database.TagCategory{
		ID:       id,
		Name:     req.Name,
		ShowName: req.ShowName,
		Scope:    types.TagScope(req.Scope),
		Enabled:  req.Enabled,
	}

	category, err := c.tagStore.UpdateCategory(ctx, newCategory)
	if err != nil {
		return nil, fmt.Errorf("failed to update category, error: %w", err)
	}

	return category, nil
}

func (c *tagComponentImpl) DeleteCategory(ctx context.Context, username string, id int64) error {
	user, err := c.userStore.FindByUsername(ctx, username)
	if err != nil {
		return fmt.Errorf("failed to get user, error: %w", err)
	}
	if !user.CanAdmin() {
		return errorx.ErrForbidden
	}

	err = c.tagStore.DeleteCategory(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete category, error: %w", err)
	}
	return nil
}
