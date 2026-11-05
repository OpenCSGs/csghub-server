//go:build !saas

package component

import (
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

// userVerifyEditionAPI keeps external identity operations out of non-SaaS editions.
type userVerifyEditionAPI interface{}

type userVerifyEdition struct{}

func (uv *userVerifyEdition) initVerifyEdition(*config.Config) error        { return nil }
func (uv *userVerifyEdition) afterVerifyUpdated(types.VerifyStatus, string) {}
