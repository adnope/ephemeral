package httpdelivery

import "context"

func (h *Handler) acquireUploadSlot(ctx context.Context) (func(), error) {
	if h.uploads == nil {
		return func() {}, nil
	}

	select {
	case h.uploads <- struct{}{}:
		return func() { <-h.uploads }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
