package server

import (
	db "deunapichincha-services/internal/app/databases"
	"deunapichincha-services/internal/app/repository"
	"deunapichincha-services/internal/config"
	"deunapichincha-services/internal/service"
	"deunapichincha-services/utils"
	"lib-shared/infrastructure/messaging_nats"
)

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
	provider.RetrievePayments()
}
