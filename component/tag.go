package component

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"opencsg.com/starhub-server/builder/store/database"
	"opencsg.com/starhub-server/common/config"
	"opencsg.com/starhub-server/component/tagparser"
)

func NewTagComponent(config *config.Config) (*TagComponent, error) {
	tc := &TagComponent{}
	tc.ts = database.NewTagStore()
	return tc, nil
}

type TagComponent struct {
	ts *database.TagStore
}

func (tc *TagComponent) AllTags(ctx context.Context) ([]database.Tag, error) {
	//TODO: query cache for tags at first
	return tc.ts.AllTags(ctx)
}

func (c *TagComponent) ClearMetaTags(ctx context.Context, namespace, name string) error {
	_, err := c.ts.SetMetaTags(ctx, namespace, name, nil)
	return err
}

func (c *TagComponent) UpdateMetaTags(ctx context.Context, tagScope database.TagScope, namespace, name, content string) ([]*database.RepositoryTag, error) {
	fileCategoryTagMap, err := tagparser.MetaTags(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse metadata, cause: %w", err)
	}
	slog.Debug("File tags parsed", slog.Any("tags", fileCategoryTagMap))

	var predefinedTags []*database.Tag
	//TODO:load from cache
	if tagScope == database.DatasetTagScope {
		predefinedTags, err = c.ts.AllDatasetTags(ctx)
	} else {
		predefinedTags, err = c.ts.AllModelTags(ctx)
	}
	if err != nil {
		slog.Error("Failed to get predefined tags", slog.Any("error", err))
		return nil, fmt.Errorf("failed to get predefined tags, cause: %w", err)
	}

	var metaTags []*database.Tag
	metaTags, err = c.prepareMetaTags(ctx, predefinedTags, fileCategoryTagMap)
	if err != nil {
		slog.Error("Failed to process tags", slog.Any("error", err))
		return nil, fmt.Errorf("failed to process tags, cause: %w", err)
	}
	var repoTags []*database.RepositoryTag
	repoTags, err = c.ts.SetMetaTags(ctx, namespace, name, metaTags)
	if err != nil {
		slog.Error("failed to set dataset's tags", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("error", err))
		return nil, fmt.Errorf("failed to set dataset's tags, cause: %w", err)
	}

	return repoTags, nil
}

func (c *TagComponent) prepareMetaTags(ctx context.Context, predefinedTags []*database.Tag, categoryTagMap map[string][]string) ([]*database.Tag, error) {
	var err error
	var tagsNeed []*database.Tag
	if len(categoryTagMap) == 0 {
		slog.Debug("No category tags to compare with predefined tags")
		return tagsNeed, nil
	}

	/*Rules for meta tags here:
	- if any tag is found in the predefined tags, accept it
	- if any tag is not found in the predefined tags but of category "Tasks", add it to "Other" category
	- if any tag is not found in the predefined tags and not of category "Tasks", ignore it
	*/
	var tagsToCreate []*database.Tag
	for category, tagNames := range categoryTagMap {
		for _, tagName := range tagNames {
			//is predefined tag, or "Other" tag created before
			if !slices.ContainsFunc(predefinedTags, func(t *database.Tag) bool {
				match := strings.EqualFold(t.Name, tagName) && (strings.EqualFold(t.Category, category) ||
					strings.EqualFold(t.Category, "Other"))

				if match {
					tagsNeed = append(tagsNeed, t)
				}
				return match
			}) {
				//all unkown tags of category "Tasks" belongs to category "Other" and will be created later
				if strings.EqualFold(category, "Tasks") {
					continue
				}
				category = "Other"
				tagsToCreate = append(tagsToCreate, &database.Tag{
					Category: category,
					Name:     tagName,
					Scope:    database.DatasetTagScope,
				})
			}
		}
	}
	//remove duplicated tag info, make sure the same tag will be created once
	tagsToCreate = slices.CompactFunc(tagsToCreate, func(t1, t2 *database.Tag) bool {
		return t1.Name == t2.Name && t1.Category == t2.Category
	})

	if len(tagsToCreate) == 0 {
		return tagsNeed, nil
	}

	err = c.ts.SaveTags(ctx, tagsToCreate)
	if err != nil {
		return nil, err
	}

	return append(tagsNeed, tagsToCreate...), nil

}

func (c *TagComponent) UpdateLibraryTags(ctx context.Context, tagScope database.TagScope, namespace, name, oldFilePath, newFilePath string) error {
	oldLibTagName := tagparser.LibraryTag(oldFilePath)
	newLibTagName := tagparser.LibraryTag(newFilePath)
	//TODO:load from cache
	var (
		allTags []*database.Tag
		err     error
	)
	if tagScope == database.DatasetTagScope {
		allTags, err = c.ts.AllDatasetTags(ctx)
	} else {
		allTags, err = c.ts.AllModelTags(ctx)
	}
	if err != nil {
		return fmt.Errorf("failed to get all tags, error: %w", err)
	}
	var oldLibTag, newLibTag *database.Tag
	for _, t := range allTags {
		if t.Name == newLibTagName {
			newLibTag = t
		}
		if t.Name == oldLibTagName {
			oldLibTag = t
		}
	}
	err = c.ts.SetLibraryTag(ctx, namespace, name, oldLibTag, newLibTag)
	if err != nil {
		slog.Error("failed to set dataset's tags", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("error", err))
		return fmt.Errorf("failed to set Library tags, cause: %w", err)
	}
	return nil
}
