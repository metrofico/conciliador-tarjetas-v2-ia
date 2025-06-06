package server

import (
	db "datafast-services/internal/app/databases"
	"datafast-services/internal/app/repository"
	"datafast-services/internal/config"
	"datafast-services/internal/service"
	"datafast-services/utils"
	"encoding/json"
	"github.com/nats-io/nats.go/jetstream"
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
	cache.LoadRestaurantAndGrupo()
	provider := service.NewApiProvider(mongoDataRepository, natsManager, cfg, cache)
	err = natsManager.EventListener.Execute("conciliador-tarjetas-services", exit, 1, "datafast.services.dispatch", "DATAFAST_SERVICES", func(msg jetstream.Msg) {
		var serviceMessage services_models.ServiceMessageDate
		err := json.Unmarshal(msg.Data(), &serviceMessage)
		if err != nil {
			utils.Error.Println("error al deserializar el mensaje para datafast service", err)
			msg.Ack()
			return
		}
		utils.Warning.Println("Procesando Datafast ------------> [ subscripcion ]")
		provider.RetrievePayments(serviceMessage.ConciliatorId, serviceMessage.HashId, serviceMessage.ProcessDate)
		msg.Ack()
	})
	if err != nil {
		utils.Error.Panic("Error creando el Listener de Nats.IO:", err)
	}
	select {}
}
