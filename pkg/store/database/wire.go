package database

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/model"
	"github.com/google/wire"
)

var WireSet = wire.NewSet(
	ProvideDatasetStore,
	ProvideModelStore,
)

func ProvideDatasetStore(db *model.DB) DatasetStore {
	return NewDatasetStore(db)
}

func ProvideModelStore(db *model.DB) ModelStore {
	return NewModelStore(db)
}
