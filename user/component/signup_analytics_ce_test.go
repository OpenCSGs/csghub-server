//go:build !ee && !saas

package component

import (
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/analytics"
	"opencsg.com/csghub-server/builder/store/database"
)

type ceSignupRecordingPublisher struct {
	events []analytics.Event
}

func (p *ceSignupRecordingPublisher) Capture(event analytics.Event) error {
	p.events = append(p.events, event)
	return nil
}

func (*ceSignupRecordingPublisher) Close() error { return nil }

func TestUserComponentDoesNotCaptureSignupSuccessInCE(t *testing.T) {
	publisher := &ceSignupRecordingPublisher{}
	component := &userComponentImpl{analytics: publisher}

	component.captureSignupSuccess(&database.User{UUID: "user-uuid"})

	require.Empty(t, publisher.events)
}
