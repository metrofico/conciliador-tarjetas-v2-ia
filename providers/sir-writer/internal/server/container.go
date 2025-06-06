package server

import (
	"encoding/json"
	"github.com/nats-io/nats.go/jetstream"
	"lib-shared/infrastructure/messaging_nats"
	"lib-shared/sir_models"
	"os"
	db "sir-writer/internal/app/databases"
	"sir-writer/internal/app/repository"
	"sir-writer/internal/config"
	"sir-writer/internal/service"
	"sir-writer/utils"
	"time"
)

var exit = make(chan os.Signal, 1)

func NewContainer(cfg config.Config) {
	mongoClient, err := db.NewMongoClient(cfg.Mongo.URI)
	if err != nil {
		utils.Error.Panic("Error creando el cliente de MongoDB: %v", err)
	}
	mongoDataRepository := repository.NewMongoDataRepository(mongoClient, cfg)
	// Conectar a NATS
	natsManager := messaging_nats.NewStartNats(cfg.Nats.URI, &messaging_nats.OptsNats{
		NameStream: "conciliador-tarjetas",
		Subjects:   []string{"sir.writer.sttransaction", "sir.writer.ventasapp"},
		MaxAge:     3 * (24 * time.Hour),
	})
	cache := service.NewDataCacheRestaurant(cfg)
	cache.LoadRestaurantAndGrupo()
	provider := service.NewApiProvider(mongoDataRepository, natsManager, cfg, cache)
	err = natsManager.EventListener.Execute("conciliador-tarjetas", exit, 1, "sir.writer.sttransaction", "SIR_WRITER", func(msg jetstream.Msg) {
		var StTransactionsWrapper sir_models.WrapperTransactions
		err := json.Unmarshal(msg.Data(), &StTransactionsWrapper)
		if err != nil {
			utils.Error.Println("error al deserializar mensaje", err)
			msg.Ack()
			return
		}
		go func(msg jetstream.Msg) {
			provider.SavePaymentsTransactions(StTransactionsWrapper, msg)
			msg.Ack()
		}(msg)

	})
	if err != nil {
		utils.Error.Panicf("Error al inicializar Listener conciliador-tarjetas - sir.writer.sttransaction: %v", err)
		return
	}
	select {}
}
