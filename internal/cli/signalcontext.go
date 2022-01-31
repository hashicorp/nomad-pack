package cli

import (
	"context"
	"os"
	"os/signal"
)

// WithInterrupt returns a Context that is done when an interrupt signal is received.
// It also returns a closer function that should be deferred for proper cleanup.
func WithInterrupt(ctx context.Context) (context.Context, func()) {

	// Create the cancellable context that we'll use when we receive an interrupt
	ctx, cancel := context.WithCancel(ctx)

	// Create the signal channel and cancel the context when we get a signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		select {
		case <-ch:
			cancel()
		case <-ctx.Done():
			return
		}
	}()

	// Return the context and a closer that cancels the context and also
	// stops any signals from coming to our channel.
	return ctx, func() {
		signal.Stop(ch)
		cancel()
	}
}
