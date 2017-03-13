package cron

import "context"

// newIDGen creates a new ID generator. The generator runs in a separate
// goroutine, so we must pass it a context.Context object
func newIDGen(ctx context.Context) chan EntryID {
	ch := make(chan EntryID)

	go func(ch chan EntryID, ctx context.Context) {
		var nextID EntryID
		for {
			select {
			case <-ctx.Done():
				return
			case ch <- nextID:
				nextID++ // did Go wrap around back to 0 if we reach max?
			}
		}
	}(ch, ctx)
	return ch
}
