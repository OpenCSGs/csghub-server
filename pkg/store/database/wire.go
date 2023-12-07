package database

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/model"
	"github.com/google/wire"
)

var WireSet = wire.NewSet(
	ProvideDatasetStore,
	ProvideModelStore,
	ProvideUserStore,
	ProvideAccessTokenStore,
	ProvideSSHKeyStore,
	ProvideOrgStore,
	ProvideMemberStore,
	ProvideNamespaceStore,
	ProvideTagStore,
	ProvideRepoStore,
)

func ProvideDatasetStore(db *model.DB) *DatasetStore {
	return NewDatasetStore(db)
}

func ProvideModelStore(db *model.DB) *ModelStore {
	return NewModelStore(db)
}

func ProvideUserStore(db *model.DB) *UserStore {
	return NewUserStore(db)
}

func ProvideAccessTokenStore(db *model.DB) *AccessTokenStore {
	return NewAccessTokenStore(db)
}

func ProvideSSHKeyStore(db *model.DB) *SSHKeyStore {
	return NewSSHKeyStore(db)
}

func ProvideOrgStore(db *model.DB) *OrgStore {
	return NewOrgStore(db)
}

func ProvideMemberStore(db *model.DB) *MemberStore {
	return NewMemberStore(db)
}

func ProvideNamespaceStore(db *model.DB) *NamespaceStore {
	return NewNamespaceStore(db)
}

func ProvideTagStore(db *model.DB) *TagStore {
	return NewTagStore(db)
}

func ProvideRepoStore(db *model.DB) *RepoStore {
	return NewRepoStore(db)
}
