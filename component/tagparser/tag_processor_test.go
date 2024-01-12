package tagparser

import (
	"context"
	"slices"
	"testing"

	"opencsg.com/csghub-server/builder/store/database"
)

type mockTagStore struct {
}

func (m *mockTagStore) AllDatasetTags(ctx context.Context) ([]*database.Tag, error) {
	tags := make([]*database.Tag, 0, 4)
	//in readme
	tags = append(tags, &database.Tag{Category: categoryNameTask, Name: "text-generation"})
	tags = append(tags, &database.Tag{Category: categoryNameTask, Name: "language-modeling"})
	tags = append(tags, &database.Tag{Category: categoryNameLanguage, Name: "zh"})
	tags = append(tags, &database.Tag{Category: categoryNameSize, Name: "100B<n<1T"})

	//not in readme
	tags = append(tags, &database.Tag{Category: categoryNameTask, Name: "text-to-image"})
	return tags, nil
}

func Test_ProcessReadme(t *testing.T) {
	ts := new(mockTagStore)
	p := NewDatasetTagProcessor(ts)
	tagsMathced, tagsNew, err := p.ProcessReadme(context.TODO(), readme)
	if err != nil {
		t.Log("Failed to process readme: ", err)
		t.FailNow()
	}

	if len(tagsMathced) != 4 {
		t.Logf("tag matched wrong number:%v", len(tagsMathced))
		for _, tag := range tagsMathced {
			t.Logf("tag matched wront content: %+v", *tag)
		}
		t.Fail()
	}

	if len(tagsNew) != 2 {
		t.Log("tag matched wrong number", len(tagsNew))
		t.Fail()
	}

	if !slices.ContainsFunc(tagsNew, func(e *database.Tag) bool {
		return e.Category == categoryNameTask && e.Name == "llm"
	}) {
		t.Logf("tags new miss 'llm'")
		t.Fail()
	}
	if !slices.ContainsFunc(tagsNew, func(e *database.Tag) bool {
		return e.Category == categoryNameTask && e.Name == "casual-lm"
	}) {
		t.Logf("tags new miss 'casual-lm'")
		t.Fail()
	}
}
func Test_processTags(t *testing.T) {
	p := new(tagProcessor)
	p.tagScope = database.DatasetTagScope

	existingCategoryTagMap := make(map[string]map[string]*database.Tag)

	//predefined Tasks tags
	existingCategoryTagMap[categoryNameTask] = make(map[string]*database.Tag)
	existingCategoryTagMap[categoryNameTask]["finance"] = &database.Tag{Name: "finance", Category: categoryNameTask}
	existingCategoryTagMap[categoryNameTask]["code"] = &database.Tag{Name: "code", Category: categoryNameTask}
	//predefined Licenses tags
	existingCategoryTagMap[categoryNameLicense] = make(map[string]*database.Tag)
	existingCategoryTagMap[categoryNameLicense]["mit"] = &database.Tag{Name: "mit", Category: categoryNameLicense}
	existingCategoryTagMap[categoryNameLicense]["apache-2.0"] = &database.Tag{Name: "apache-2.0", Category: categoryNameLicense}
	//predefined Libraries tags
	existingCategoryTagMap[categoryNameFramework] = make(map[string]*database.Tag)
	existingCategoryTagMap[categoryNameFramework]["pytorch"] = &database.Tag{Name: "pytorch", Category: categoryNameFramework}

	categoryTagMap := make(map[string][]string)
	categoryTagMap[categoryNameTask] = append(categoryTagMap[categoryNameTask], "finance", "code", "mit") //should create an "task" tag "mit"
	categoryTagMap[categoryNameLicense] = append(categoryTagMap[categoryNameLicense], "mit")              //should match this one
	categoryTagMap["Unkown"] = append(categoryTagMap["Unknown"], "mit")                                   //should igore this one

	tagsMatched, tagsToCreate := p.processTags(existingCategoryTagMap, categoryTagMap)
	if len(tagsMatched) != 3 {
		t.Log("tagsMatched wrong number")
		t.FailNow()
	}

	if len(tagsToCreate) != 1 {
		t.Log("tagsToCreate wrong number", len(tagsToCreate))
		t.FailNow()
	}

	newTag := tagsToCreate[0]
	if newTag.Name != "mit" || newTag.Category != categoryNameTask || newTag.BuiltIn == true {
		t.Logf("tagsToCreate wrong:%v", newTag)
		t.FailNow()
	}
}
