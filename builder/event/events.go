package event

import (
	"log/slog"
)

type EventPublisher struct {
	Connector *NatsConnector
}

// NewEventPublisher creates a new instance of EventPublisher
func NewEventPublisher() *EventPublisher {
	return &EventPublisher{
		Connector: defaultNats,
	}
}

// Publish sends a message to the specified subject
func (ec *EventPublisher) Publish(subject string, message []byte) error {
	var err error
	if subject == "" {
		subject = ec.Connector.Subject
	}
	err = ec.Connector.Conn.Publish(subject, message)
	if err == nil {
		slog.Info("Published event", slog.Any("event", message))
		return nil
	}
	slog.Error("Failed to publish event, send to retry channel", slog.Any("error", err))
	//send to retry channel
	err = ec.Connector.Conn.Publish(ec.Connector.RetrySubject, message)
	if err != nil {
		slog.Error("Failed to publish event to retry channel", slog.Any("error", err), slog.Any("event", message))
	}
	return err
}
