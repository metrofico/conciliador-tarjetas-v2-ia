package server

import (
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v3"
	"github.com/nats-io/nats.go/jetstream"
	"lib-shared/infrastructure/messaging_nats"
	"lib-shared/reports_models"
	utils2 "lib-shared/utils"
	"os"
	db "report-system/internal/app/databases"
	"report-system/internal/app/repository"
	"report-system/internal/config"
	"report-system/internal/service"
	"report-system/utils"
	"strings"
	"time"
)

var exit = make(chan os.Signal, 1)
var StreamName = "conciliador-tarjetas-report"

type ErrorResponse struct {
	Error string `json:"error"`
}

type SuccessResponse struct {
	Id      string `json:"id"`
	Hash    string `json:"hash"`
	Message string `json:"message"`
}

var (
	BucketServicesProgress = "CONCILIATORS_PROGRESS"
)
var mongoDataRepository *repository.MongoDataRepository
var cfgGlobal config.Config
var natsManager *messaging_nats.NatsStarter

func NewContainer(cfg config.Config) {
	cfgGlobal = cfg
	mongoClient, err := db.NewMongoClient(cfg.Mongo.URI)
	if err != nil {
		utils.Error.Panicf("Error creando el cliente de MongoDB: %v", err)
	}
	mongoDataRepository = repository.NewMongoDataRepository(mongoClient, cfg)
	// Conectar a NATS
	natsManager = messaging_nats.NewStartNats(cfg.Nats.URI, &messaging_nats.OptsNats{
		NameStream: StreamName,
		Subjects:   []string{"*.data.report"},
		MaxAge:     24 * time.Hour,
	})
	reportProvider := service.NewApiProvider(mongoDataRepository, natsManager, cfg)
	err = natsManager.EventListener.Execute(StreamName, exit, 1, "new.data.report", "NEW_REPORTS", func(msg jetstream.Msg) {
		var report reports_models.ReportConciliator
		err := json.Unmarshal(msg.Data(), &report)
		if err != nil {
			msg.Ack()
			utils.Error.Printf("Error decoding the report data: %v\n", err)
			return
		}
		reportProvider.CreateReport(report)
		msg.Ack()
	})
	if err != nil {
		utils.Error.Panic("Error al creando el nats", err)
	}
	err = natsManager.EventListener.Execute(StreamName, exit, 1, "add.data.report", "ADD_REPORTS", func(msg jetstream.Msg) {
		var addReport reports_models.ReportData
		err := json.Unmarshal(msg.Data(), &addReport)
		if err != nil {
			msg.Ack()
			utils.Error.Printf("Error decoding the report data: %v\n", err)
			return
		}
		reportProvider.AddToReport(addReport)
		msg.Ack()
	})
	if err != nil {
		utils.Error.Panic("Error al creando el nats", err)
	}
	err = natsManager.EventListener.Execute(StreamName, exit, 1, "started.data.report", "STARTED_REPORT", func(msg jetstream.Msg) {
		var started reports_models.StartedReport
		err := json.Unmarshal(msg.Data(), &started)
		if err != nil {
			msg.Ack()
			utils.Error.Printf("Error decoding the report data: %v\n", err)
			return
		}
		reportProvider.StartedReport(started)
		msg.Ack()
	})
	if err != nil {
		utils.Error.Panic("Error al creando el nats", err)
	}
	err = natsManager.EventListener.Execute(StreamName, exit, 1, "completed.data.report", "COMPLETED_REPORT", func(msg jetstream.Msg) {
		var completed reports_models.CompletedReport
		err := json.Unmarshal(msg.Data(), &completed)
		if err != nil {
			msg.Ack()
			utils.Error.Printf("Error decoding the report data: %v\n", err)
			return
		}
		reportProvider.CompletedReport(completed)
		msg.Ack()
	})
	if err != nil {
		utils.Error.Panic("Error al creando el nats", err)
	}
	app := fiber.New()
	group := app.Group("/api/payment-conciliator")
	group.Get("/details/:id", handlerDetailOperation)
	group.Post("/payments/:id", handlerTemp)
	utils.Info.Println("Servidor en http://" + cfg.HttpServer)
	app.Listen(cfg.HttpServer)
}
func handlerTemp(c fiber.Ctx) error {
	return nil
}
func handlerDetailOperation(c fiber.Ctx) error {
	id := c.Params("id", "")
	if utils2.IsEmptyString(id) {
		return c.Status(fiber.StatusBadRequest).JSON(&ErrorResponse{
			Error: "Debes especificar el id de la transacción",
		})
	}
	report, err := mongoDataRepository.FindById(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(&ErrorResponse{
			Error: fmt.Sprintf("Error al obtener el report: %v", err),
		})
	}
	if report == nil {
		return c.Status(fiber.StatusBadRequest).JSON(&ErrorResponse{
			Error: fmt.Sprintf("No se encontró ningun detalle para este reporte"),
		})
	}
	dataReports, err := mongoDataRepository.FindDataById(id)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(&ErrorResponse{
			Error: fmt.Sprintf("Error al obtener los datos del report: %v", err),
		})
	}
	if dataReports == nil {
		dataReports = make([]reports_models.ReportDataBson, 0)
	}
	request := report.Request
	completed := report.GetCompletedAtFormatted(cfgGlobal.TimeZone)
	if strings.EqualFold(completed, "No ha finalizado") {
		hash := request.Hash()
		progress, _ := natsManager.GetValueBucket(BucketServicesProgress, hash)
		completed = "En progreso: " + progress + "%"
	}
	entries := &reports_models.ReportEntriesJsonResponse{
		Inserted: 0,
		Updated:  0,
		Ignored:  0,
	}
	if report.Entries != nil {
		entries.Inserted = report.Entries.Inserted
		entries.Updated = report.Entries.Updated
		entries.Ignored = report.Entries.Ignored
	}
	reportResponse := reports_models.ReportConciliatorJsonResponse{
		ConciliatorId: report.ConciliatorId,
		CreatedAt:     report.GetCreatedAtFormatted(cfgGlobal.TimeZone),
		StartedAt:     report.GetStartedAtFormatted(cfgGlobal.TimeZone),
		CompletedAt:   completed,
		ElapsedTime:   report.GetElapsedTimeFormatted(),
		Entries:       entries,
		Request:       report.Request,
		ReportData:    dataReports,
	}
	return c.Status(fiber.StatusOK).JSON(reportResponse)
}
