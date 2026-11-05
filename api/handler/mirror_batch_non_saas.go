//go:build !saas

package handler

import "opencsg.com/csghub-server/component"

func configureBatchMirrorHandler(handler *MirrorHandler, _ component.MirrorComponent) error {
	// Keep the shared handler shape available to edition-specific files while
	// making the intentionally disabled capability explicit in CE and EE.
	_ = handler.batchMirror
	return nil
}
