package nats

import (
	"context"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Connect dials the NATS server at the given URL and returns the connection.
func Connect(url string) (*nats.Conn, error) {
	return nats.Connect(url)
}

// EnsureStream creates or updates the RECORDS stream in JetStream.
// The stream captures subjects used for record domain events.
func EnsureStream(js jetstream.JetStream) error {
	cfg := jetstream.StreamConfig{
		Name:      "RECORDS",
		Subjects:  []string{"record.ingested", "record.indexed"},
		Retention: jetstream.LimitsPolicy,
		MaxAge:    7 * 24 * time.Hour,
		Storage:   jetstream.FileStorage,
		Replicas:  1,
	}

	_, err := js.CreateOrUpdateStream(context.Background(), cfg)
	return err
}
