package nats

import (
	"context"

	"github.com/nats-io/nats.go/jetstream"
)

// Publisher implements record.EventPublisher using NATS JetStream.
type Publisher struct {
	js jetstream.JetStream
}

// NewPublisher returns a Publisher backed by the given JetStream context.
func NewPublisher(js jetstream.JetStream) *Publisher {
	return &Publisher{js: js}
}

// Publish sends data to the given subject via JetStream.
// It implements the record.EventPublisher interface.
func (p *Publisher) Publish(ctx context.Context, subject string, data []byte) error {
	_, err := p.js.Publish(ctx, subject, data)
	return err
}
