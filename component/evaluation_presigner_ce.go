//go:build !ee && !saas

package component

import "opencsg.com/csghub-server/common/config"

// initPresigner returns nil in CE build (no StorageGatewayComponent available).
func initPresigner(config *config.Config) presignURLer {
	return nil
}
