package service

import (
	"bytes"
	"context"
	db "datafast-services/internal/app/databases"
	"datafast-services/internal/app/repository"
	"datafast-services/internal/config"
	"datafast-services/internal/models"
	"datafast-services/utils"
	"encoding/json"
	"encoding/xml"
	"fmt"
	uuid2 "github.com/google/uuid"
	"io"
	"lib-shared/infrastructure/messaging_nats"
	lib_mapper "lib-shared/mapper"
	"lib-shared/reports_models"
	"lib-shared/sir_models"
	utils3 "lib-shared/utils"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ApiProviderDatafast struct {
	mongoRepository *repository.MongoDataRepository
	country         string
	cfg             config.Config
	natsManager     *messaging_nats.NatsStarter
	cache           *DataCacheRestaurant
}

type Envelope struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    Body     `xml:"Body"`
}

type Body struct {
	XMLName              xml.Name             `xml:"Body"`
	ConsultaDBALResponse ConsultaDBALResponse `xml:"ConsultaDBALResponse"`
}

type ConsultaDBALResponse struct {
	XMLName            xml.Name `xml:"ConsultaDBALResponse"`
	ConsultaDBALResult string   `xml:"ConsultaDBALResult"`
}

func NewApiProvider(mongoRepository *repository.MongoDataRepository, natsManager *messaging_nats.NatsStarter, cfg config.Config, cache *DataCacheRestaurant) *ApiProviderDatafast {
	return &ApiProviderDatafast{
		mongoRepository: mongoRepository,
		natsManager:     natsManager,
		cfg:             cfg,
		cache:           cache,
	}
}

var (
	BucketNameLocker       = "CONCILIATOR_LOCKS"
	BucketServicesProgress = "CONCILIATORS_PROGRESS"
)

func (provider *ApiProviderDatafast) RetrievePayments(conciliatorId, hash string, processDate time.Time) {
	// Obtener datos de configuración
	provider.SetProgress(hash, fmt.Sprintf("%.2f", 0.0))
	// Leer la fecha para el filtro
	startedEventMessage := reports_models.StartedReport{
		ConciliatorId: conciliatorId,
		StartedTime:   time.Now(),
	}
	utils.Info.Println("started.data.report " + conciliatorId)
	provider.natsManager.EventSender.SendMsgBytesJson("started.data.report", startedEventMessage)
	// Establecer la zona horaria
	location, err := time.LoadLocation(provider.cfg.TimeZone)
	if err != nil {
		utils.Error.Println("Error al cargar la zona horaria: ", err)
		return
	}
	// Obtener la fecha y hora actual en la zona horaria especificada
	localTime := processDate.In(location)
	utils.Info.Println("Procesando conciliador en fecha en formato AAAA-MM-DD -> " + localTime.Format("2006-01-02"))
	startExecutor := time.Now()
	// Obtener la fecha de "ayer"

	// Crear una nueva conexión a la base de datos
	conn, err := db.NewSQLServerConnection(provider.cfg.SqlServerSir.JDBC)
	if err != nil {
		msg := fmt.Sprintf("error conectando a la base de datos: %v\n", err)
		utils.Error.Printf(msg)
		provider.SendErrorConciliator(conciliatorId, msg, nil)
		provider.Complete(conciliatorId, startExecutor, 0, 0, 0)
		return
	}
	defer conn.Close()
	defer provider.natsManager.AcquiredUnlock(BucketNameLocker, hash)
	// Ejecutar la consulta y obtener múltiples registros como un slice de mapas
	query := "select Url_servicio, usuario, contrasena from Configuracion_WebServices where Nombre = 'API_SOAP_DATAFAST'"
	var url, usuario, clave string
	err = conn.QueryRow(query).Scan(&url, &usuario, &clave)
	if err != nil {
		errMsg := fmt.Sprintf("No se puede continuar con la conciliación: error ejecutando la  consulta: %v", err)
		utils.Error.Println(errMsg)
		provider.SendErrorConciliator(conciliatorId, errMsg, nil)
		provider.Complete(conciliatorId, startExecutor, 0, 0, 0)
		return
	}
	// Retornar el regex que permitirá validar para no subir totalmente lo que devuelva datafast y solo (lo que corresponde)

	datafastMetadata := &DatafastMetadata{
		Uri:      url,
		Username: usuario,
	}
	// Variables de ejemplo
	fecha := localTime.Format("20060102")
	tipo := "1"
	claveWS := "81304D8C-82DF-4CD2-AA90-FC70BC6C90AC"

	// Construir el cuerpo de la solicitud SOAP
	data := fmt.Sprintf(`<![CDATA[<DBAL><FECHA>%s</FECHA><TIPO>%s</TIPO><USUARIO>%s</USUARIO><CLAVE>%s</CLAVE><CLAVEWS>%s</CLAVEWS></DBAL>]]>`,
		fecha, tipo, usuario, clave, claveWS)

	requestBody := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:tem="http://tempuri.org/">
                                    <soapenv:Header/>
                                    <soapenv:Body>
                                        <tem:ConsultaDBAL>
                                            <tem:xml>%s</tem:xml>
                                        </tem:ConsultaDBAL>
                                    </soapenv:Body>
                                </soapenv:Envelope>`, data)

	// Crear un request HTTP POST
	req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(requestBody)))
	if err != nil {
		errMsg := fmt.Sprintf("No se puede continuar con la conciliación: Error creando la solicitud: %v", err)
		utils.Error.Println(errMsg)
		provider.SendErrorConciliator(conciliatorId, errMsg, datafastMetadata)
		provider.Complete(conciliatorId, startExecutor, 0, 0, 0)
		return
	}

	// Añadir cabeceras
	req.Header.Add("Content-Type", "text/xml;charset=\"utf-8\"")
	req.Header.Add("Accept", "text/xml")
	req.Header.Add("Cache-Control", "no-cache")
	req.Header.Add("Pragma", "no-cache")
	req.Header.Add("SOAPAction", "http://tempuri.org/IServicio/ConsultaDBAL")

	// Enviar la solicitud
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}
	resp, err := client.Do(req)
	if err != nil {
		errMsg := fmt.Sprintf("No se puede continuar con la conciliación: error inicializando la petición Post por SOAP: %v", err)
		utils.Error.Println(errMsg)
		provider.SendErrorConciliator(conciliatorId, errMsg, datafastMetadata)
		provider.Complete(conciliatorId, startExecutor, 0, 0, 0)
		return
	}
	defer resp.Body.Close()
	// Leer la respuesta
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		errorBody, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("error al obtener transacciones de datafast StatusCode: %d, Body: %s", resp.StatusCode, string(errorBody))
		utils.Error.Println(errMsg)
		provider.SendErrorConciliator(conciliatorId, errMsg, datafastMetadata)
		provider.Complete(conciliatorId, startExecutor, 0, 0, 0)
		return
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		errMsg := fmt.Sprintf("[datafast][ReadAll] No se puede continuar con la conciliación error al interpretar la respuesta de la petición: %v", err)
		utils.Error.Println(errMsg)
		provider.SendErrorConciliator(conciliatorId, errMsg, datafastMetadata)
		provider.Complete(conciliatorId, startExecutor, 0, 0, 0)
		return
	}

	// Procesar la respuesta SOAP
	var envelope Envelope
	err = xml.Unmarshal(body, &envelope)
	if err != nil {
		errMsg := fmt.Sprintf("No se puede continuar con la conciliación: error procesando el XML SOAP: %v", err)
		utils.Error.Println(errMsg)
		provider.SendErrorConciliator(conciliatorId, errMsg, datafastMetadata)
		provider.Complete(conciliatorId, startExecutor, 0, 0, 0)
		return
	}

	// Extraer el contenido del `ConsultaDBALResult` que es un XML embebido
	consultaDBALXML := envelope.Body.ConsultaDBALResponse.ConsultaDBALResult

	// Quitar los caracteres de escape de CDATA (como `&lt;`, `&gt;`)
	consultaDBALXML = xmlEscapeToNormal(consultaDBALXML)

	// Deserializar el XML embebido en la estructura de transacciones
	var paymentResponse models.PaymentResponse
	err = xml.Unmarshal([]byte(consultaDBALXML), &paymentResponse)
	if err != nil {
		errMsg := fmt.Sprintf("No se puede continuar con la conciliación: error procesando el XML embebido: %v", err)
		utils.Error.Println(errMsg)
		provider.SendErrorConciliator(conciliatorId, errMsg, datafastMetadata)
		provider.Complete(conciliatorId, startExecutor, 0, 0, 0)
		return
	}
	// Recorrer los registros y procesarlos
	paymentsNormalize := make([]lib_mapper.Payment, 0)
	createdAt := time.Now()
	originalRecords := *paymentResponse.Records
	records := make([]models.PaymentData, 0)
	regexCompile := regexp.MustCompile(provider.cfg.TidsDatafast)
	provider.SetProgress(hash, "Etapa 1/3 - Filtrando transacciones")
	for _, payment := range originalRecords {
		if regexCompile.MatchString(payment.TID) && payment.Bin != "179131" {
			records = append(records, payment)
		}
	}
	paymentResponse.Records = nil
	sizePayments := len(records)
	utils.Warning.Printf("[datafast] se procesaran %d pagos", sizePayments)
	lastReportedProgress := 0.0
	for i, payment := range records {
		codCadena := provider.cache.getCodCadena(payment.MID)
		// Mapeo de los campos
		merchantId := provider.cache.getMerchantId(payment.MID)
		if utils3.IsEmptyString(merchantId) {
			errMsg := fmt.Sprintf("[datafast] No se encontró el merchantId (No existe o está desactivado) con el MID '%s', autorización '%s',referencia '%s'", payment.MID, payment.Autorizacion, payment.Referencia)
			utils.Error.Println(errMsg)
			itemMetadata := &DatafastItemMetadata{
				Uri:      url,
				Username: usuario,
				MID:      payment.MID,
				TID:      payment.TID,
			}
			provider.SendErrorConciliator(conciliatorId, errMsg, itemMetadata)
			continue
		}
		transformed := sir_models.StTransactions{
			MerchantId:         merchantId,
			FechaTransaccion:   formatFecha(payment.FechaTrx),
			HoraTransaccion:    formatHora(payment.FechaTrx),
			Estado:             "1",
			NumeroLote:         payment.Lote,
			FaceValue:          strconv.FormatFloat(payment.VTotal, 'f', 2, 64),
			IdGrupoTarjeta:     provider.cache.getIdGrupoTarjeta(codCadena, payment.Bin),
			IdAdquirente:       "DATAFAST",
			NumeroTarjetaMask:  "1234XXXXXXXXXXX1234",
			NumeroAutorizacion: payment.Autorizacion,
			NumeroReferencia:   payment.Referencia,
			TipoTransaccion:    "01",
			ResultadoExterno:   "00",
			TipoSwitch:         provider.cache.getTipoSwitch(payment.MID),
			OrigenTransaccion:  provider.cache.getOrigen(payment.MID),
			Sistema:            provider.cache.getSistema(payment.MID),
		}
		paymentsNormalize = append(paymentsNormalize, lib_mapper.Payment{
			ConciliatorId: conciliatorId,
			StoreId:       merchantId,
			UniqueId:      transformed.GetUniqueId(),
			Hash:          transformed.GetTransactionHash(),
			Provider:      "DATAFAST",
			CreatedAt:     createdAt,
			Data: lib_mapper.PaymentData{
				Input:  payment,
				Output: transformed,
			},
		})
		currentIteration := i + 1
		totalProgress := float64(currentIteration*100) / float64(sizePayments)
		// Redondear al decimal más cercano con 1 decimal (por seguridad)
		progressStep := 0.2
		roundedProgress := math.Floor(totalProgress/progressStep) * progressStep
		if roundedProgress > lastReportedProgress {
			lastReportedProgress = roundedProgress
			provider.SetProgress(hash, "Etapa 2/3 - "+fmt.Sprintf("%.2f", totalProgress))
		}
	}
	paymentResponse.Records = nil
	batches := batchProcessPayments(paymentsNormalize, 250)
	var entriesInserted, entriesUpdated, entriesIgnored uint32

	totalBatches := len(batches)
	// i = 0 [ i + 1 = 1 ]
	// totalBatches = 10 elements
	//
	// totalBatches == 100
	// (i + 1) = ?
	// Regla de 3 para obtener el progreso al 100%
	lastReportedProgress = 0.0
	for i, batch := range batches {
		utils.Info.Println(fmt.Sprintf("[Datafast-Insert-Mongo] Procesando batch #%d\n", i+1))
		StTransactionsData := make([]sir_models.Transaction, 0)
		var uniqueIds []string
		for _, payment := range batch {
			uniqueIds = append(uniqueIds, payment.UniqueId)
		}
		// Enviar proceso a SIR con sus respectivas operations
		findPaymentsHash, _ := provider.mongoRepository.FindPaymentsHash(uniqueIds)
		for _, payment := range batch {
			operation := "INSERT"
			for _, paymentHash := range findPaymentsHash {
				if strings.EqualFold(payment.UniqueId, paymentHash.UniqueId) {
					if strings.EqualFold(payment.Hash, paymentHash.Hash) {
						operation = "IGNORE"
						entriesIgnored++
						continue
					}
					operation = "UPDATE"
					entriesUpdated++
					break
				}
			}
			if operation == "INSERT" {
				entriesInserted++
			}

			if operation == "IGNORE" {
				continue
			}
			StTransactionsData = append(StTransactionsData, sir_models.Transaction{
				OperationType: operation,
				Data:          payment.Data.Output,
			})
		}
		saveErr := provider.mongoRepository.SaveBulkModel(batch)
		if saveErr != nil {
			utils.Error.Println(fmt.Sprintf("[Datafast-Insert-Mongo] Error al procesar batch #%d\n", i+1), saveErr)
		}
		if len(StTransactionsData) > 0 {
			uuid, _ := uuid2.NewV7()
			batchAsBytes, _ := json.Marshal(sir_models.WrapperTransactions{
				Id:           uuid.String(),
				Transactions: StTransactionsData,
			})
			provider.natsManager.EventSender.SendMsgBytes("sir.writer.sttransaction", batchAsBytes)
		} else {
			utils.Info.Println(fmt.Sprintf("[Datafast-Publish] no hay datos para modificar (Empty) (ya existe o no hay cambios) #%d\n", i+1))
		}
		currentIteration := i + 1
		totalProgress := float64(currentIteration*100) / float64(totalBatches)
		progressStep := 0.2
		roundedProgress := math.Floor(totalProgress/progressStep) * progressStep
		if roundedProgress > lastReportedProgress {
			lastReportedProgress = roundedProgress
			provider.SetProgress(hash, "Etapa 3/3 - "+fmt.Sprintf("%.2f", totalProgress))
		}
	}
	provider.SetProgress(hash, fmt.Sprintf("%.2f", 100.0))
	elapsedExecutor := time.Since(startExecutor)
	utils.Info.Println("[datafast] pagos completado, la tarea tardó " + utils.FormatDuration(elapsedExecutor))
	completedEventMessage := reports_models.CompletedReport{
		ConciliatorId: conciliatorId,
		CompletedAt:   time.Now(),
		ElapsedTime:   uint32(elapsedExecutor.Seconds()),
		Entries: reports_models.EntriesCompletedReport{
			Inserted: entriesInserted,
			Updated:  entriesUpdated,
			Ignored:  entriesIgnored,
		},
	}
	provider.natsManager.EventSender.SendMsgBytesJson("completed.data.report", completedEventMessage)
}

func batchProcessPayments(payments []lib_mapper.Payment, batchSize int) [][]lib_mapper.Payment {
	var batches [][]lib_mapper.Payment
	for i := 0; i < len(payments); i += batchSize {
		end := i + batchSize
		if end > len(payments) {
			end = len(payments)
		}
		batches = append(batches, payments[i:end])
	}
	return batches
}
func formatFecha(fecha string) string {
	parsedDate, err := time.Parse("20060102150405", fecha)
	if err != nil {
		utils.Error.Println("[formatFecha] Error parsing date:", err)
	}

	// Formatear la fecha en "Y-m-d"
	return parsedDate.Format("2006-01-02")
}
func (provider *ApiProviderDatafast) Complete(conciliatorId string, startTime time.Time, inserted, updated, ignored uint32) {
	elapsedExecutor := time.Since(startTime)
	provider.SetProgress(conciliatorId, fmt.Sprintf("%.2f", 100.0))
	completedEventMessage := reports_models.CompletedReport{
		ConciliatorId: conciliatorId,
		CompletedAt:   time.Now(),
		ElapsedTime:   uint32(elapsedExecutor.Seconds()),
		Entries: reports_models.EntriesCompletedReport{
			Inserted: inserted,
			Updated:  updated,
			Ignored:  ignored,
		},
	}
	provider.natsManager.EventSender.SendMsgBytesJson("completed.data.report", completedEventMessage)
}
func (provider *ApiProviderDatafast) SetProgress(conciliatorId string, progressAsString string) {
	kv := provider.natsManager.CreateIfNotExistBucket(BucketServicesProgress)
	if kv != nil {
		utils.Info.Println("datafast progress " + progressAsString)
		_, err := kv.Put(context.Background(), conciliatorId, []byte(progressAsString))
		if err != nil {
			utils.Error.Printf("Error al cargar guardar el progreso en Nats KV: conciliatorId: %s, %v\n", conciliatorId, err)
		}
	}
}
func formatHora(fecha string) string {
	parsedDate, err := time.Parse("20060102150405", fecha)
	if err != nil {
		utils.Error.Println("[formatHora] Error parsing date:", err)
	}

	// Formatear la fecha en "Y-m-d"
	return parsedDate.Format("150405")
}

type DatafastMetadata struct {
	Uri      string `json:"uri"`
	Username string `json:"username"`
}
type DatafastItemMetadata struct {
	Uri      string `json:"uri"`
	Username string `json:"username"`
	MID      string `json:"mid"`
	TID      string `json:"tid"`
}

func (provider *ApiProviderDatafast) SendInfoConciliator(conciliatorId, message string, metadata interface{}) {
	msg := reports_models.ReportData{
		ConciliatorId: conciliatorId,
		Type:          "INFO",
		Message:       message,
		Metadata: reports_models.Metadata{
			Content: metadata,
		},
		CreatedAt: time.Now(),
	}
	provider.natsManager.EventSender.SendMsgBytesJson("add.data.report", msg)
}
func (provider *ApiProviderDatafast) SendErrorConciliator(conciliatorId, message string, metadata any) {
	msg := reports_models.ReportData{
		ConciliatorId: conciliatorId,
		Type:          "ERROR",
		Message:       message,
		Metadata: reports_models.Metadata{
			Content: metadata,
		},
		CreatedAt: time.Now(),
	}
	provider.natsManager.EventSender.SendMsgBytesJson("add.data.report", msg)
}
func xmlEscapeToNormal(escaped string) string {
	replacer := strings.NewReplacer(
		"&lt;", "<",
		"&gt;", ">",
		"&amp;", "&",
		"&quot;", "\"",
		"&apos;", "'",
	)
	return replacer.Replace(escaped)
}
