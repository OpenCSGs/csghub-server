package component

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type UserPhoneComponent interface {
	NeedPhoneChange() bool
	CanChangePhone(ctx context.Context, user *database.User, newPhone string) (bool, error)
	UpdatePhone(ctx context.Context, uid string, req types.UpdateUserPhoneRequest) error
	SendSMSCode(ctx context.Context, uid string, req types.SendSMSCodeRequest) (*types.SendSMSCodeResponse, error)
	SendPublicSMSCode(ctx context.Context, req types.SendPublicSMSCodeRequest) (*types.SendSMSCodeResponse, error)
	VerifyPublicSMSCode(ctx context.Context, req types.VerifyPublicSMSCodeRequest) error
}
