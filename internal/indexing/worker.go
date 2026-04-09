package indexing

import "context"

// IndexingWorker adapts Pipeline.Process to the nats.MessageHandler signature,
// making it usable as a consumer callback without exposing Pipeline directly.
type IndexingWorker struct {
	pipeline *Pipeline
}

// NewIndexingWorker returns an IndexingWorker backed by the given Pipeline.
func NewIndexingWorker(pipeline *Pipeline) *IndexingWorker {
	return &IndexingWorker{pipeline: pipeline}
}

// Handle processes a raw NATS message payload by delegating to the pipeline.
// It returns ErrPermanent for unrecoverable failures and ErrTransient for
// retriable ones — matching the contract expected by the NATS consumer.
func (w *IndexingWorker) Handle(ctx context.Context, data []byte) error {
	return w.pipeline.Process(ctx, data)
}
