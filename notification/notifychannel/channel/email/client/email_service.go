package client

import (
	"opencsg.com/csghub-server/common/types"
)

type EmailService interface {
	Send(req types.EmailReq) error
}
