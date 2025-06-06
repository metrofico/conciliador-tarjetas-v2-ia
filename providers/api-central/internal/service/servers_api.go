package service

import (
	"api-starter-jobs/internal/app/repository"
	"api-starter-jobs/internal/config"
	"lib-shared/infrastructure/messaging_nats"
)

type ApiProviderDatafast struct {
	mongoRepository *repository.MongoDataRepository
	country         string
	cfg             config.Config
	natsManager     *messaging_nats.NatsStarter
}

func NewApiProvider(mongoRepository *repository.MongoDataRepository, natsManager *messaging_nats.NatsStarter, cfg config.Config) *ApiProviderDatafast {
	return &ApiProviderDatafast{
		mongoRepository: mongoRepository,
		natsManager:     natsManager,
		cfg:             cfg,
	}
}

func (provider *ApiProviderDatafast) CreateJobK8s() {

}
