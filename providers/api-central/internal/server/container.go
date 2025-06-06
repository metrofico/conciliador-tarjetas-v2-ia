package server

import (
	db "api-starter-jobs/internal/app/databases"
	"api-starter-jobs/internal/app/repository"
	"api-starter-jobs/internal/config"
	"api-starter-jobs/utils"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"lib-shared/infrastructure/messaging_nats"
	"lib-shared/reports_models"
	"lib-shared/services_models"
	utils2 "lib-shared/utils"
	"strings"
	"time"
)

var nats *messaging_nats.NatsStarter
var location *time.Location
var repositoryData *repository.MongoDataRepository

var (
	BucketNameLocker       = "CONCILIATOR_LOCKS"
	BucketServicesProgress = "CONCILIATORS_PROGRESS"
)

func NewContainer(cfg config.Config) {
	mongoClient, err := db.NewMongoClient(cfg.Mongo.URI)
	if err != nil {
		utils.Error.Panic("Error creando el cliente de MongoDB: %v", err)
	}
	repositoryData = repository.NewMongoDataRepository(mongoClient, cfg)
	// Conectar a NATS
	nats = messaging_nats.NewStartNats(cfg.Nats.URI, &messaging_nats.OptsNats{
		NameStream: "conciliador-tarjetas-services",
		Subjects:   []string{"*.services.dispatch"},
		MaxAge:     1 * time.Hour,
	})
	location, err = time.LoadLocation(cfg.TimeZone)
	if err != nil {
		utils.Error.Panic("Error al cargar la zona horaria: ", err)
	}

	app := fiber.New()
	group := app.Group("/api/payment-conciliator")
	group.Post("/generate-conciliator", handlerProcessConciliador)
	utils.Info.Println("Servidor en http://" + cfg.HttpServer)
	app.Listen(cfg.HttpServer)
}

type RequestBody struct {
	Fecha   string `json:"fecha"`
	Service string `json:"service"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type SuccessResponse struct {
	Id      string `json:"id"`
	Hash    string `json:"hash"`
	Message string `json:"message"`
}

func handlerProcessConciliador(c *fiber.Ctx) error {
	var body RequestBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "JSON inválido",
		})
	}

	// Parsear la fecha para validar que sea real y esté en el formato correcto
	parsedDate, err := time.ParseInLocation("2006-01-02", body.Fecha, location)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Fecha inválida, debe tener formato AAAA-MM-DD y ser real",
		})
	}
	if utils2.IsEmptyString(body.Service) {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Servicio no disponible, por favor asegurate de utilizar los siguientes [kiosco,datafast,smartlink,etc]",
		})
	}
	serviceConciliator := strings.TrimSpace(strings.ToLower(body.Service))
	switch serviceConciliator {
	case "kiosco":
	case "datafast":
	case "smartlink":
	default:
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "Servicio no disponible, por favor asegurate de utilizar los siguientes [kiosco,datafast,smartlink,etc]",
		})
	}

	// Save Report
	uid, _ := uuid.NewV7()
	uidAsString := uid.String()
	// Formatear como 20060102 - Dato a Insert
	formateada := parsedDate.Format("20060102")
	request := reports_models.Request{
		Service: strings.ToUpper(serviceConciliator),
		Date:    strings.TrimSpace(body.Fecha),
	}
	report := reports_models.ReportConciliator{
		ConciliatorId: uidAsString,
		CreatedAt:     time.Now(),
		StartedAt:     nil,
		CompletedAt:   nil,
		Request:       request,
	}
	hash := request.Hash()
	locked, err := nats.AcquiredLock(BucketNameLocker, hash)
	if err != nil {
		utils.Error.Println("Error al acquirar el locked: %v", err)
	}
	if !locked {
		progress, err := nats.GetValueBucket(BucketServicesProgress, hash)
		if err != nil {
			utils.Error.Printf("Error al obtener el progreso de la operación: %v", err)
		}

		utils.Error.Printf("Ya existe un proceso en progreso al [ %s%% ] para la fecha [ %s ] del servicio [ %s ]", progress, body.Fecha, serviceConciliator)
		return c.Status(fiber.StatusConflict).JSON(ErrorResponse{
			Error: fmt.Sprintf("Existe una transacción en ejecución con progreso [ %s%% ] para la fecha [ %s ] del servicio [ %s ], espera a que finalice para continuar", progress, body.Fecha, serviceConciliator),
		})
	}

	message := services_models.ServiceMessageDate{ConciliatorId: uidAsString, ProcessDate: parsedDate.UTC(), HashId: hash}
	nats.EventSender.SendMsgBytesJson("new.data.report", report)
	nats.EventSender.SendMsgBytesJson(fmt.Sprintf("%s.services.dispatch", serviceConciliator), message)
	return c.Status(fiber.StatusOK).JSON(SuccessResponse{
		Id:      uidAsString,
		Hash:    hash,
		Message: fmt.Sprintf("Obteniendo datos de conciliación para el servicio [%s] en la fecha [%s](AAAA-MM-DD)", strings.ToUpper(serviceConciliator), formateada),
	})
}
