package messaging_nats

import (
	"context"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type NatsModuleManager struct {
	natsClient *nats.Conn
	jetStream  jetstream.JetStream
	mainStream jetstream.Stream
	ctx        context.Context
}

func NewManager(
	natsClient *nats.Conn,
	jetStream jetstream.JetStream,
	orderStream jetstream.Stream,
	ctx context.Context,
) *NatsModuleManager {
	return &NatsModuleManager{
		natsClient: natsClient,
		jetStream:  jetStream,
		mainStream: orderStream,
		ctx:        ctx,
	}
}

func (manager NatsModuleManager) GetClient() *nats.Conn {
	return manager.natsClient
}

func (manager NatsModuleManager) GetJetStream() jetstream.JetStream {
	return manager.jetStream
}

func (manager NatsModuleManager) GetMainStream() jetstream.Stream {
	return manager.mainStream
}

func (manager NatsModuleManager) GetContext() context.Context {
	return manager.ctx
}
