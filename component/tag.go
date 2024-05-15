package component

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/component/tagparser"
)

func NewTagComponent(config *config.Config) (*TagComponent, error) {
	tc := &TagComponent{}
	tc.ts = database.NewTagStore()
	tc.rs = database.NewRepoStore()
	tc.sensitiveChecker = NewSensitiveComponent(config)
	return tc, nil
}

type TagComponent struct {
	ts               *database.TagStore
	rs               *database.RepoStore
	sensitiveChecker SensitiveChecker
}

func (tc *TagComponent) AllTags(ctx context.Context) ([]database.Tag, error) {
	// TODO: query cache for tags at first
	return tc.ts.AllTags(ctx)
}

func (c *TagComponent) ClearMetaTags(ctx context.Context, namespace, name string) error {
	_, err := c.ts.SetMetaTags(ctx, namespace, name, nil)
	return err
}

func (c *TagComponent) UpdateMetaTags(ctx context.Context, tagScope database.TagScope, namespace, name, content string) ([]*database.RepositoryTag, error) {
	var tp tagparser.TagProcessor
	// TODO:load from cache

	if tagScope == database.DatasetTagScope {
		tp = tagparser.NewDatasetTagProcessor(c.ts)
	} else if tagScope == database.ModelTagScope {
		tp = tagparser.NewModelTagProcessor(c.ts)
	} else {
		// skip tag process for code and space now
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
			pass, _ := c.sensitiveChecker.CheckText(ctx, string(sensitive.ScenarioNicknameDetection), t.Name)
			return !pass
		})
	}

	err = c.ts.SaveTags(ctx, tagToCreate)
	if err != nil {
		slog.Error("Failed to save tags", slog.Any("error", err))
		return nil, fmt.Errorf("failed to save tags, cause: %w", err)
	}
	metaTags := append(tagsMatched, tagToCreate...)
	var repoTags []*database.RepositoryTag
	repoTags, err = c.ts.SetMetaTags(ctx, namespace, name, metaTags)
	if err != nil {
		slog.Error("failed to set dataset's tags", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("error", err))
		return nil, fmt.Errorf("failed to set dataset's tags, cause: %w", err)
	}

	return repoTags, nil
}

func (c *TagComponent) UpdateLibraryTags(ctx context.Context, tagScope database.TagScope, namespace, name, oldFilePath, newFilePath string) error {
	oldLibTagName := tagparser.LibraryTag(oldFilePath)
	newLibTagName := tagparser.LibraryTag(newFilePath)
	// TODO:load from cache
	var (
		allTags []*database.Tag
		err     error
	)
	if tagScope == database.DatasetTagScope {
		allTags, err = c.ts.AllDatasetTags(ctx)
	} else if tagScope == database.ModelTagScope {
		allTags, err = c.ts.AllModelTags(ctx)
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
	err = c.ts.SetLibraryTag(ctx, namespace, name, newLibTag, oldLibTag)
	if err != nil {
		slog.Error("failed to set dataset's tags", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("failed to set Library tags, cause: %w", err)
	}
	return nil
}

func (c *TagComponent) UpdateRepoTagsByCategory(ctx context.Context, tagScope database.TagScope, repoID int64, category string, tagNames []string) error {
	allTags, err := c.ts.AllTagsByScopeAndCategory(ctx, tagScope, category)
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
	oldTagIDs, err = c.rs.TagIDs(ctx, repoID, category)
	if err != nil {
		return fmt.Errorf("failed to get old tag ids, error: %w", err)
	}
	return c.ts.UpsertRepoTags(ctx, repoID, oldTagIDs, tagIDs)
}
