package nats

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/ricaguroc/toi-motor/internal/indexing"
)

// MessageHandler processes a single NATS message payload.
// Returning ErrPermanent causes the message to be acked (no redelivery).
// Returning any other non-nil error causes a nak (redelivery up to MaxDeliver).
// Returning nil causes a normal ack.
type MessageHandler func(ctx context.Context, data []byte) error

// StartConsumer creates a durable JetStream consumer and processes messages
// from the given stream, filter subject, and handler.
// It blocks until ctx is cancelled, then returns nil.
func StartConsumer(
	ctx context.Context,
	js jetstream.JetStream,
	streamName, consumerName, subject string,
	handler MessageHandler,
) error {
	cfg := jetstream.ConsumerConfig{
		Durable:       consumerName,
		FilterSubject: subject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       60 * time.Second,
		MaxDeliver:    10,
		DeliverPolicy: jetstream.DeliverAllPolicy,
	}

	consumer, err := js.CreateOrUpdateConsumer(ctx, streamName, cfg)
	if err != nil {
		return err
	}

	slog.Info("nats consumer started", "stream", streamName, "consumer", consumerName, "subject", subject)

	for {
		msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
		if ctx.Err() != nil {
			slog.Info("nats consumer shutting down", "consumer", consumerName)
			return nil
		}
		if err != nil {
			// Timeout waiting for next message — loop and check ctx again.
			continue
		}

		if handlerErr := handler(ctx, msg.Data()); handlerErr != nil {
			if errors.Is(handlerErr, indexing.ErrPermanent) {
				slog.Error("permanent indexing error, acking to discard", "err", handlerErr)
				if ackErr := msg.Ack(); ackErr != nil {
					slog.Warn("ack failed after permanent error", "err", ackErr)
				}
			} else {
				slog.Warn("transient indexing error, nacking for redelivery", "err", handlerErr)
				if nakErr := msg.Nak(); nakErr != nil {
					slog.Warn("nak failed after transient error", "err", nakErr)
				}
			}
		} else {
			if ackErr := msg.Ack(); ackErr != nil {
				slog.Warn("ack failed", "err", ackErr)
			}
		}
	}
}
