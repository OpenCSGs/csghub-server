package types

import "opencsg.com/csghub-server/runner/utils"

type Validator func(value string) bool

var SubscribeKeyWithEventPush = map[string]Validator{
	"STARHUB_SERVER_RUNNER_PUBLIC_DOMAIN": utils.ValidUrl,
}
