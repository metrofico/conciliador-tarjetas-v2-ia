package service

import (
	"lib-shared/infrastructure/messaging_nats"
	"lib-shared/reports_models"
	"lib-shared/utils"
	"report-system/internal/app/repository"
	"report-system/internal/config"
)

type ReportService struct {
	mongoRepository *repository.MongoDataRepository
	country         string
	cfg             config.Config
	natsManager     *messaging_nats.NatsStarter
}

func NewApiProvider(mongoRepository *repository.MongoDataRepository, natsManager *messaging_nats.NatsStarter, cfg config.Config) *ReportService {
	return &ReportService{
		mongoRepository: mongoRepository,
		natsManager:     natsManager,
		cfg:             cfg,
	}
}

func (provider *ReportService) CreateReport(report reports_models.ReportConciliator) {
	// Obtener datos de configuración
	err := provider.mongoRepository.CreateReport(report)
	if err != nil {
		utils.Info.Println("save report error", err)
	}
}
func (provider *ReportService) AddToReport(dataReport reports_models.ReportData) {
	// Obtener datos de configuración
	err := provider.mongoRepository.AddToReport(dataReport)
	if err != nil {
		utils.Info.Println("save data at report error", err)
	}
}
func (provider *ReportService) StartedReport(dataReport reports_models.StartedReport) {
	// Obtener datos de configuración
	err := provider.mongoRepository.StartedReport(dataReport)
	if err != nil {
		utils.Info.Println("save data at report error", err)
	}
}
func (provider *ReportService) CompletedReport(dataReport reports_models.CompletedReport) {
	// Obtener datos de configuración
	err := provider.mongoRepository.CompletedReport(dataReport)
	if err != nil {
		utils.Info.Println("save data at report error", err)
	}
}
