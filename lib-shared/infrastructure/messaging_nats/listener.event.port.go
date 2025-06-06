package messaging_nats

import (
	"github.com/nats-io/nats.go/jetstream"
	"os"
)

type ListenerEventPort interface {
	Execute(
		stream string,
		exit <-chan os.Signal,
		batch int,
		eventName string,
		durable string,
		execute func(msg jetstream.Msg),
	) error
}
