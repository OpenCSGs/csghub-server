package component

import (
	"context"

	"opencsg.com/starhub-server/builder/store/database"
	"opencsg.com/starhub-server/common/config"
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
