package messaging_nats

import (
	"context"
	"fmt"
	"github.com/nats-io/nats.go/jetstream"
	"lib-shared/utils"
	"os"
	"strings"
	"time"
)

type ListenerEventAdapter struct {
	NatsModuleManager *NatsModuleManager
}

func NewListenerEventAdapter(manager *NatsModuleManager) ListenerEventPort {
	return &ListenerEventAdapter{
		NatsModuleManager: manager,
	}
}

func (listener *ListenerEventAdapter) getContext() context.Context {
	return listener.NatsModuleManager.GetContext()
}

func (listener *ListenerEventAdapter) Execute(
	streamName string,
	exit <-chan os.Signal,
	batch int,
	eventName string,
	durable string,
	execute func(msg jetstream.Msg),
) error {
	eventName = strings.ToLower(eventName)
	stream, err := listener.NatsModuleManager.GetJetStream().Stream(context.Background(), streamName)
	if err != nil {
		return fmt.Errorf("error al acceder al stream '%s': %w", streamName, err)
	}
	consume, err := stream.CreateOrUpdateConsumer(
		context.Background(),
		jetstream.ConsumerConfig{
			Durable:        durable,
			Description:    "",
			DeliverPolicy:  jetstream.DeliverAllPolicy,
			AckPolicy:      jetstream.AckExplicitPolicy,
			FilterSubjects: []string{eventName},
			AckWait:        1 * time.Hour,
		})

	if err != nil {
		return err
	}
	utils.Info.Println(fmt.Sprintf("listening event [%s]", eventName))

	go func() {
		for {
			select {
			case <-exit:
				utils.Info.Println("Se ha dejado de recibir datos")
				break
			default:
				{
					msg, err := consume.Fetch(batch)
					if err != nil {
						utils.Error.Println("Error en el consumidor de nats: ", err.Error())
						continue
					}
					if msg.Error() != nil {
						utils.Error.Println(msg.Error().Error())
						continue
					}

					for msg := range msg.Messages() {
						if err != nil {
							utils.Error.Println("[pre-execute] error al obtener mensaje en el consumidor ", eventName, " err: ", err)
						}
						select {
						case _ = <-exit:
							utils.Info.Println("Se ha dejado de recibir datos")
							break
						default:
							{
								utils.Info.Println("[debug-nats] receiving new message (pre-execute)")
								execute(msg)
								if err != nil {
									utils.Error.Println("[post-execute] error al obtener mensaje en el consumidor ", eventName, " err: ", err)
								}
							}
						}
					}
				}
			}

		}
	}()

	return err
}
