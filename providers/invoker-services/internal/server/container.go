package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	db "invoker-services/internal/app/databases"
	"invoker-services/internal/app/repository"
	"invoker-services/internal/config"
	"invoker-services/utils"
	"io"
	"lib-shared/infrastructure/messaging_nats"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var nats *messaging_nats.NatsStarter
var location *time.Location
var repositoryData *repository.MongoDataRepository

type RequestPayload struct {
	Fecha   string `json:"fecha"`
	Service string `json:"service"`
}

type ResponseOK struct {
	ID        string    `json:"_id"`
	CreatedAt time.Time `json:"createdAt"`
	Request   struct {
		Service string `json:"service"`
		Date    string `json:"date"`
	} `json:"request"`
	Response struct {
		Status int `json:"status"`
	} `json:"response"`
}

type ResponseError struct {
	CreatedAt time.Time `json:"createdAt"`
	Request   struct {
		Service string `json:"service"`
		Date    string `json:"date"`
	} `json:"request"`
	Response struct {
		Status int    `json:"status"`
		Body   string `json:"body"` // compressed JSON
	} `json:"response"`
}

func NewContainer(cfg config.Config) {
	mongoClient, err := db.NewMongoClient(cfg.Mongo.URI)
	if err != nil {
		utils.Error.Panic("Error creando el cliente de MongoDB: %v", err)
	}
	repositoryData = repository.NewMongoDataRepository(mongoClient, cfg)
	// Conectar a NATS
	nats = messaging_nats.NewStartNats(cfg.Nats.URI, nil)
	location, err = time.LoadLocation(cfg.TimeZone)
	if err != nil {
		utils.Error.Panic("Error al cargar la zona horaria: ", err)
	}
	now := time.Now().In(location)
	offset := cfg.TimeOffset
	if offset == "" {
		offset = "0d"
	}

	// Validar y parsear el offset como días
	duration, err := parseDayDuration(offset)
	if err != nil {
		fmt.Printf("Error en TIME_OFFSET: %v\n", err)
		return
	}

	modifiedDate := now.Add(duration).Format("2006-01-02")
	// Preparar payload
	payload := RequestPayload{
		Fecha:   modifiedDate,
		Service: cfg.ConciliadorServicePush,
	}
	sendHttpRequest(payload, cfg.ApiCentralConciliador)
	repositoryData.Close()
	nats.Close()
}

func sendHttpRequest(payload RequestPayload, apiCentralDns string) {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("Error serializando JSON:", err)
		return
	}

	// Enviar petición
	resp, err := http.Post(apiCentralDns, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		fmt.Println("Error en la petición HTTP:", err)
		return
	}
	defer resp.Body.Close()

	// Leer respuesta
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error leyendo respuesta:", err)
		return
	}

	// Simular log con struct
	if resp.StatusCode == http.StatusOK {
		result := ResponseOK{
			CreatedAt: time.Now(),
		}
		result.Request.Service = payload.Service
		result.Request.Date = payload.Fecha
		result.Response.Status = resp.StatusCode

		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println("Respuesta OK:")
		fmt.Println(string(out))
	} else {
		// Minificar JSON
		var minified bytes.Buffer
		if err := json.Compact(&minified, body); err != nil {
			fmt.Println("Error minificando JSON:", err)
			return
		}

		result := ResponseError{
			CreatedAt: time.Now(),
		}
		result.Request.Service = payload.Service
		result.Request.Date = payload.Fecha
		result.Response.Status = resp.StatusCode
		result.Response.Body = minified.String()

		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println("Respuesta Error:")
		fmt.Println(string(out))
	}
}

func parseDayDuration(input string) (time.Duration, error) {
	input = strings.TrimSpace(input)
	if !strings.HasSuffix(input, "d") {
		return 0, fmt.Errorf("formato inválido, debe terminar en 'd'")
	}

	valueStr := strings.TrimSuffix(input, "d")
	days, err := strconv.Atoi(valueStr)
	if err != nil {
		return 0, fmt.Errorf("valor de días inválido: %v", err)
	}

	hours := days * 24
	return time.Duration(hours) * time.Hour, nil
}
