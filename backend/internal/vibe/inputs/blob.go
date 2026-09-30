package inputs

import (
	"context"
	"io"
	"sync"
	"time"
)

type boundedBody struct {
	io.ReadCloser
	once   sync.Once
	stop   func() bool
	cancel context.CancelFunc
	err    error
}

func (b *boundedBody) Close() error {
	b.stop()
	b.cancel()
	b.once.Do(func() { b.err = b.ReadCloser.Close() })
	return b.err
}
func (s *Repository) openBlob(ctx context.Context, key string, timeout time.Duration) (io.ReadCloser, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	body, _, err := s.Blobs.OpenObject(ctx, key)
	if err != nil {
		cancel()
		return nil, err
	}
	b := &boundedBody{ReadCloser: body, cancel: cancel}
	// Install the callback before sharing the wrapper; it closes the underlying
	// body directly, so cancellation cannot race initialization of b.stop.
	b.stop = context.AfterFunc(ctx, func() { b.once.Do(func() { b.err = body.Close() }) })
	return b, nil
}
