package handler

import "opencsg.com/csghub-server/builder/analytics"

type recordingPublisher struct {
	events []analytics.Event
	err    error
}

func (p *recordingPublisher) Capture(event analytics.Event) error {
	p.events = append(p.events, event)
	return p.err
}

func (p *recordingPublisher) Close() error { return nil }
