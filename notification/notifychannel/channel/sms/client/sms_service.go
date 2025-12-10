package client

import "opencsg.com/csghub-server/common/types"

type SMSService interface {
	Send(req types.SMSReq) error
}
