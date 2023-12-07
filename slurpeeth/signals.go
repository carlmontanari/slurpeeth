package slurpeeth

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

var onlyOneSignalHandler = make(chan struct{}) //nolint: gochecknoglobals

// SignalHandledContext returns a context that will be canceled if a SIGINT or SIGTERM is
// received.
func SignalHandledContext(
	logf func(f string, a ...interface{}),
) (context.Context, context.CancelFunc) {
	// panics when called twice, this way there can only be one signal handled context
	close(onlyOneSignalHandler)

	ctx, cancel := context.WithCancel(context.Background())

	sigs := make(chan os.Signal, 2) //nolint:gomnd

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		logf("received signal '%s', canceling context", sig)

		cancel()

		<-sigs
		logf("received signal '%s', exiting program", sig)

		os.Exit(130) //nolint:gomnd
	}()

	return ctx, cancel
}
