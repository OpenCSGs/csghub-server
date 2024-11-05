package component

import (
	"context"
	"testing"
	"time"

	"opencsg.com/csghub-server/common/tests"
	"opencsg.com/csghub-server/common/types"
)

func TestSetRelationDatasetsAndPrompts(t *testing.T) {
	cfg := InitTestDB(t)

	mc, err := NewModelComponent(cfg)
	if err != nil {
		t.Fatalf("failed to create model component: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := types.RelationDatasets{
		CurrentUser: tests.CurrentUser,
		Namespace:   tests.TestModelNamespace,
		Name:        tests.TestModelName,
		Datasets:    []string{"wanghh2003/ds7"},
	}

	err = mc.SetRelationDatasets(ctx, req)
	if err != nil {
		t.Errorf("failed to set relation datasets: %v", err)
	}
}
