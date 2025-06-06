package server

import (
	"encoding/json"
	"github.com/nats-io/nats.go/jetstream"
	db "kioscos-services/internal/app/databases"
	"kioscos-services/internal/app/repository"
	"kioscos-services/internal/config"
	"kioscos-services/internal/service"
	"kioscos-services/utils"
	"lib-shared/infrastructure/messaging_nats"
	"lib-shared/services_models"
	"os"
)

var exit = make(chan os.Signal, 1)

func NewContainer(cfg config.Config) {
	mongoClient, err := db.NewMongoClient(cfg.Mongo.URI)
	if err != nil {
		utils.Error.Panic("Error creando el cliente de MongoDB: %v", err)
	}
	mongoDataRepository := repository.NewMongoDataRepository(mongoClient, cfg)
	// Conectar a NATS
	natsManager := messaging_nats.NewStartNats(cfg.Nats.URI, nil)
	cache := service.NewDataCacheRestaurant(cfg)
	cache.LoadRestaurant()
	provider := service.NewApiProvider(mongoDataRepository, natsManager, cfg, cache)
	err = natsManager.EventListener.Execute("conciliador-tarjetas-services", exit, 1, "kiosco.services.dispatch", "KIOSCO_SERVICES", func(msg jetstream.Msg) {
		var serviceMessage services_models.ServiceMessageDate
		err := json.Unmarshal(msg.Data(), &serviceMessage)
		if err != nil {
			utils.Error.Println("error al deserializar el mensaje para kiosco service", err)
			msg.Ack()
			return
		}
		provider.RetrievePayments(serviceMessage.ConciliatorId, serviceMessage.HashId, serviceMessage.ProcessDate)
		msg.Ack()
	})
	if err != nil {
		utils.Error.Panic("Error creando el Listener de Nats.IO:", err)
	}
	select {}
}
