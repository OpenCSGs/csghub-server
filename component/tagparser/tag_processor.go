package tagparser

import (
	"context"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/builder/store/database"
)

type DatasetTagStore interface {
	AllDatasetTags(ctx context.Context) ([]*database.Tag, error)
}

type ModelTagStore interface {
	AllModelTags(ctx context.Context) ([]*database.Tag, error)
}

type TagProcessor interface {
	ProcessReadme(ctx context.Context, content string) (tagsMatched, tagsNew []*database.Tag, err error)
	ProcessFramework(ctx context.Context, fileName string) (*database.Tag, error)
}

// make sure tagProcessor implements TagProcessor
var _ TagProcessor = (*tagProcessor)(nil)

type tagProcessor struct {
	existingTags func(ctx context.Context) ([]*database.Tag, error)
	tagScope     database.TagScope
}

func NewDatasetTagProcessor(ts DatasetTagStore) TagProcessor {
	p := new(tagProcessor)
	p.existingTags = ts.AllDatasetTags
	p.tagScope = database.DatasetTagScope
	return p
}

func NewModelTagProcessor(ts ModelTagStore) TagProcessor {
	p := new(tagProcessor)
	p.existingTags = ts.AllModelTags
	p.tagScope = database.ModelTagScope
	return p
}

func (p *tagProcessor) ProcessReadme(ctx context.Context, content string) (tagsMatched, tagsNew []*database.Tag, err error) {
	metaCategoryTags, err := MetaTags(content)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse metadata, cause: %w", err)
	}
	slog.Debug("File tags parsed", slog.Any("tags", metaCategoryTags))

	var existingTags []*database.Tag
	existingTags, err = p.existingTags(ctx)
	if err != nil {
		slog.Error("Failed to get exiting tags", slog.Any("error", err))
		return nil, nil, fmt.Errorf("failed to get existing tags, cause: %w", err)
	}

	existingCategoryTags := p.mapCategoryTag(existingTags)
	tagsMatched, tagsNew = p.processTags(existingCategoryTags, metaCategoryTags)
	return
}

func (p *tagProcessor) ProcessFramework(ctx context.Context, fileName string) (*database.Tag, error) {
	//TODO:move framework tag processing from component package to here
	return nil, nil
}

// processTags compare tags input with existing tags, return tags matched and tags new
func (p *tagProcessor) processTags(existingCategoryTagMap map[string]map[string]*database.Tag,
	categoryTagMap map[string][]string) ([]*database.Tag, []*database.Tag) {
	var tagsMatched []*database.Tag
	var tagsToCreate []*database.Tag
	for category, tagNames := range categoryTagMap {
		existingTaskTags, found := existingCategoryTagMap[category]
		if !found {
			continue
		}
		for _, tagName := range tagNames {
			if tag, ok := existingTaskTags[tagName]; !ok {
				tagsToCreate = append(tagsToCreate, &database.Tag{
					Name:     tagName,
					Category: category,
					Scope:    p.tagScope,
					BuiltIn:  false, // new tag is absolutly not built-in
					Group:    "",    // keep empty
				})
			} else {
				tagsMatched = append(tagsMatched, tag)
			}
		}

	}

	return tagsMatched, tagsToCreate
}

func (p *tagProcessor) mapCategoryTag(tags []*database.Tag) map[string]map[string]*database.Tag {
	predefinedCategoryTagMap := make(map[string]map[string]*database.Tag)
	for _, tag := range tags {
		var ok bool
		var tags map[string]*database.Tag
		if tags, ok = predefinedCategoryTagMap[tag.Category]; !ok {
			tags = make(map[string]*database.Tag)
			predefinedCategoryTagMap[tag.Category] = tags
		}
		tags[tag.Name] = tag
	}
	return predefinedCategoryTagMap
}
