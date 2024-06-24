package event

import (
	"log"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"opencsg.com/csghub-server/common/config"
)

var defaultNats *NatsConnector

// NatsConnector struct to hold the connection
type NatsConnector struct {
	Conn          *nats.Conn
	Subject       string
	RetrySubject  string
	NotifySubject string
	SyncInterval  int //in minutes
}

// NewNatsConnector initializes a new connection to the NATS server
func InitNats(cfg *config.Config) {
	nc, err := nats.Connect(cfg.Accounting.NatsURL,
		nats.Timeout(10*time.Second),
		nats.ReconnectWait(10*time.Second),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		log.Fatal("Failed to connect to NATS: ", err)
	} else {
		slog.Info("Nats connection created")
	}

	defaultNats = &NatsConnector{
		Conn:          nc,
		Subject:       cfg.Event.FeeSendSubject,
		RetrySubject:  cfg.Event.FeeSendRetrySubject,
		NotifySubject: cfg.Event.NoBalanceReceiveSubject,
		SyncInterval:  cfg.Event.SyncInterval,
	}
}

// Close method to close the NATS connection
func (n *NatsConnector) Close() {
	if n.Conn != nil {
		n.Conn.Close()
	}
}
