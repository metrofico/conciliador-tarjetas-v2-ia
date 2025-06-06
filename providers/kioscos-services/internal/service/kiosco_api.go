package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	uuid2 "github.com/google/uuid"
	"io"
	db "kioscos-services/internal/app/databases"
	"kioscos-services/internal/app/repository"
	"kioscos-services/internal/config"
	"kioscos-services/internal/models"
	"kioscos-services/utils"
	"lib-shared/infrastructure/messaging_nats"
	lib_mapper "lib-shared/mapper"
	"lib-shared/reports_models"
	"lib-shared/sir_models"
	utils3 "lib-shared/utils"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	BucketNameLocker       = "CONCILIATOR_LOCKS"
	BucketServicesProgress = "CONCILIATORS_PROGRESS"
)

type ApiProviderDatafast struct {
	mongoRepository *repository.MongoDataRepository
	country         string
	cfg             config.Config
	natsManager     *messaging_nats.NatsStarter
	cache           *DataCacheRestaurant
}

type IpAddressRestaurant struct {
	Direccion string
	Puerto    string
	Email     string
	Clave     string
	IdLocal   string
}

func NewApiProvider(mongoRepository *repository.MongoDataRepository, natsManager *messaging_nats.NatsStarter, cfg config.Config, cache *DataCacheRestaurant) *ApiProviderDatafast {
	return &ApiProviderDatafast{
		mongoRepository: mongoRepository,
		natsManager:     natsManager,
		cfg:             cfg,
		cache:           cache,
	}
}
func (provider *ApiProviderDatafast) RetrievePayments(conciliatorId, hash string, processDate time.Time) {
	// Obtener datos de configuración

	// Leer la fecha para el filtro

	// Establecer la zona horaria
	location, err := time.LoadLocation(provider.cfg.TimeZone)
	if err != nil {
		utils.Error.Println("Error al cargar la zona horaria: ", err)
	}

	// Obtener la fecha y hora actual en la zona horaria especificada
	startExecutor := time.Now()
	// Obtener la fecha de "ayer"
	localTime := processDate.In(location)
	utils.Info.Println("Procesando conciliador en fecha en formato AAAA-MM-DD -> " + localTime.Format("2006-01-02"))
	dateFormat := localTime.Format("20060102")
	// Crear una nueva conexión a la base de datos
	sql := db.SQLServerConnection{}
	conn, err := sql.NewSQLServerConnection(provider.cfg.SqlServerSir.JDBC)
	if err != nil {
		utils.Error.Printf("error conectando a la base de datos: %v\n", err)
	}
	defer conn.Close()
	defer provider.natsManager.AcquiredUnlock(BucketNameLocker, hash)

	// Ejecutar la consulta y obtener múltiples registros como un slice de mapas
	query := "select distinct(idLocal), direccion, puerto, email, clave from KioskoWs"

	rows, err := conn.Query(query)
	if err != nil {
		utils.Error.Printf("error ejecutando la consulta: %v\n", err)
	}
	ipAddressRestaurants := make([]*IpAddressRestaurant, 0)
	for rows.Next() {
		var direccion, puerto, email, clave, idLocal string
		err = rows.Scan(&idLocal, &direccion, &puerto, &email, &clave)
		if err != nil {
			utils.Error.Printf("error scaning data la consulta: %v\n", err)
			continue
		}
		ipAddressRestaurants = append(ipAddressRestaurants, &IpAddressRestaurant{
			Direccion: direccion,
			Puerto:    puerto,
			Email:     email,
			Clave:     clave,
			IdLocal:   idLocal,
		})
	}
	maxConcurrent := 15
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	totalRestaurantsLen := len(ipAddressRestaurants)
	utils.Warning.Printf("se procesarán %d restaurantes:", totalRestaurantsLen)
	startedEventMessage := reports_models.StartedReport{
		ConciliatorId: conciliatorId,
		StartedTime:   time.Now(),
	}

	// Calcular Progreso de Tarea
	tasksDone := make(chan int, totalRestaurantsLen)
	totalTasks := totalRestaurantsLen
	go func() {
		completed := 0
		for range tasksDone {
			completed++
			pending := totalTasks - completed
			progress := float64(completed) / float64(totalTasks) * 100
			provider.SetProgress(hash, fmt.Sprintf("%.2f", progress))
			utils.Info.Printf("Progreso: %.2f%% - Completadas: %d - Pendientes: %d",
				progress, completed, pending)
		}
	}()
	provider.natsManager.EventSender.SendMsgBytesJson("started.data.report", startedEventMessage)

	var entriesInserted, entriesUpdated, entriesIgnored atomic.Uint32
	for i, ipAddrRest := range ipAddressRestaurants {
		wg.Add(1)
		go func(ip *IpAddressRestaurant, index int) {
			defer wg.Done()
			sem <- struct{}{}        // Adquiere un slot
			defer func() { <-sem }() // Libera el slot
			defer func() {
				if r := recover(); r != nil {
					utils.Error.Printf("[task-batch kiosco] (%s) Recovered from panic: %v", ip.Direccion, r)
				}
				// Notificar tarea completada
				tasksDone <- 1
			}()
			utils.Info.Printf("start process restaurante %s\n", ip.Direccion)
			inserted, ignored, updated := provider.processRestaurant(ip, dateFormat, conciliatorId)
			entriesInserted.Add(inserted)
			entriesUpdated.Add(updated)
			entriesIgnored.Add(ignored)
		}(ipAddrRest, i)
	}
	wg.Wait()
	provider.SetProgress(hash, "100.00")
	elapsedExecutor := time.Since(startExecutor)
	inserted := entriesInserted.Load()
	updated := entriesUpdated.Load()
	ignored := entriesIgnored.Load()
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
	utils.Info.Printf("[kiosco] pagos completado, la tarea tardó %s con registros inserted %d, updated %d, ignored %d", utils.FormatDuration(elapsedExecutor),
		inserted, updated, ignored)
}
func (provider *ApiProviderDatafast) SetProgress(conciliatorId string, progressAsString string) {
	kv := provider.natsManager.CreateIfNotExistBucket(BucketServicesProgress)
	if kv != nil {
		_, err := kv.Put(context.Background(), conciliatorId, []byte(progressAsString))
		if err != nil {
			utils.Error.Printf("Error al cargar guardar el progreso en Nats KV: conciliatorId: %s, %v\n", conciliatorId, err)
		}
	}
}
func (provider *ApiProviderDatafast) SendErrorConciliator(conciliatorId, message string, metadata interface{}) {
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
func (provider *ApiProviderDatafast) processRestaurant(ipAddressRestaurant *IpAddressRestaurant, dateFormat, conciliatorId string) (uint32, uint32, uint32) {
	direccion := ipAddressRestaurant.Direccion
	puerto := ipAddressRestaurant.Puerto
	httpAddress := createHttpAddress(direccion, puerto)
	tokenAcceso, err := GenerateTokenApi(httpAddress, ipAddressRestaurant.Email, ipAddressRestaurant.Clave)
	if err != nil {
		errMsg := fmt.Sprintf("error al obtener el token en el server: %s details: %v", httpAddress, err)
		utils.Error.Println(errMsg)
		provider.SendErrorConciliator(conciliatorId, errMsg, &ipAddressRestaurant)
		return 0, 0, 0
	}
	if utils.IsEmptyString(tokenAcceso) {
		return 0, 0, 0
	}

	route := "/api/reportes/ventas-switch?"
	paramsUrl := `&fechainicio=` + dateFormat + `&fechafin=` + dateFormat
	uri := httpAddress + route + paramsUrl
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		errMsg := fmt.Sprintf("[kiosco][/api/reportes/ventas-switch] error al crear la petición: %v, server %s", err, httpAddress)
		utils.Error.Println(errMsg)
		provider.SendErrorConciliator(conciliatorId, errMsg, &ipAddressRestaurant)
		return 0, 0, 0
	}

	req.Header.Set("Authorization", "Bearer "+tokenAcceso)
	req.Header.Set("verify", "false")
	client := &http.Client{Timeout: 3 * time.Minute}

	startElapseHttp := time.Now()
	utils.Info.Printf("[kiosco][client.Do][WAITING] solicitando datos a uri: %s\n", httpAddress)
	resp, err := client.Do(req)
	elapseHttp := time.Since(startElapseHttp)
	utils.Info.Printf("[kiosco][client.Do][SUCCESS] solicitud datos devueltos: %s, took: %s\n", httpAddress, utils.FormatDuration(elapseHttp))

	if err != nil {
		errMsg := fmt.Sprintf("[kiosco] error conectando a la petición: %v, server %s", err, httpAddress)
		utils.Error.Println(errMsg)
		provider.SendErrorConciliator(conciliatorId, errMsg, &ipAddressRestaurant)
		return 0, 0, 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		errorBody, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("error al obtener transacciones en el server: %s StatusCode: %d, Body: %s", httpAddress, resp.StatusCode, string(errorBody))
		utils.Error.Println(errMsg)
		provider.SendErrorConciliator(conciliatorId, errMsg, &ipAddressRestaurant)
		return 0, 0, 0
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		errMsg := fmt.Sprintf("[kiosco][ReadAll] error al interpretar la respuesta de la petición: %v, server %s", err, httpAddress)
		utils.Error.Println(errMsg)
		provider.SendErrorConciliator(conciliatorId, errMsg, &ipAddressRestaurant)
		return 0, 0, 0
	}

	var paymentResponse []models.PaymentData
	if err := json.Unmarshal(body, &paymentResponse); err != nil {
		errMsg := fmt.Sprintf("[kiosco][Unmarshal] error al deserializar el contenido de la respuesta: %v, server %s", err, httpAddress)
		utils.Error.Println(errMsg)
		provider.SendErrorConciliator(conciliatorId, errMsg, &ipAddressRestaurant)
		return 0, 0, 0
	}

	paymentsNormalize := make([]lib_mapper.Payment, 0)
	utils.Warning.Printf("registros iniciales totales a procesar de pagos %d", len(paymentResponse))
	createdAt := time.Now()
	paymentsUnprocessable := 0
	for _, payment := range paymentResponse {
		merchantId := provider.cache.getMerchantId(ipAddressRestaurant.IdLocal)
		if utils3.IsEmptyString(merchantId) {
			paymentsUnprocessable++
			errMsg := fmt.Sprintf("[kiosco][merchantId] El MerchantId para la autorización '%s', IdLocal '%s',referencia '%s' está vacia", payment.NumeroAutorizacion, ipAddressRestaurant.IdLocal, payment.NumeroReferencia)
			utils.Error.Println(errMsg)
			provider.SendErrorConciliator(conciliatorId, errMsg, &ipAddressRestaurant)
			continue
		}
		transformed := sir_models.StTransactions{
			MerchantId:         provider.cache.getMerchantId(payment.MerchantId),
			FechaTransaccion:   payment.FechaTransaccion,
			HoraTransaccion:    formatHora(payment.HoraTransaccion),
			Estado:             payment.Estado,
			NumeroLote:         payment.NumeroLote,
			FaceValue:          payment.FaceValue,
			IdGrupoTarjeta:     formatearTipoTarjeta(payment.IdGrupoTarjeta),
			IdAdquirente:       payment.IdAdquirente,
			NumeroTarjetaMask:  payment.NumeroTarjetaMask,
			NumeroAutorizacion: payment.NumeroAutorizacion,
			NumeroReferencia:   payment.NumeroReferencia,
			TipoTransaccion:    payment.TipoTransaccion,
			ResultadoExterno:   payment.ResultadoExterno,
			TipoSwitch:         utils.StringToInt(payment.TipoSwitch),
			OrigenTransaccion:  utils.StringToInt(payment.OrigenTransaccion),
			Sistema:            "MAXPOINT",
		}
		paymentsNormalize = append(paymentsNormalize, lib_mapper.Payment{
			UniqueId:      transformed.GetUniqueId(),
			Hash:          transformed.GetTransactionHash(),
			StoreId:       ipAddressRestaurant.IdLocal,
			ConciliatorId: conciliatorId,
			Provider:      "KIOSKO",
			CreatedAt:     createdAt,
			Data: lib_mapper.PaymentData{
				Input:  payment,
				Output: transformed,
			},
		})
	}
	utils.Warning.Printf("registros reales procesables %d", len(paymentsNormalize))
	utils.Warning.Printf("registros no procesables %d", paymentsUnprocessable)
	batches := batchProcessPayments(paymentsNormalize, 250)
	var inserted, ignored, updated uint32
	processed := 0
	for i, batch := range batches {
		batchSize := len(batch)
		processed = processed + batchSize
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
						//output := payment.Data.Output
						//utils.Warning.Printf("hashed duplicated %s, autorization %s, fecha %s, card %s", payment.Hash, output.NumeroAutorizacion, output.FechaTransaccion, output.NumeroTarjetaMask)
						operation = "IGNORE"
						ignored++
						break
					}
					operation = "UPDATE"
					updated++
					break
				}
			}
			if operation == "INSERT" {
				inserted++
			}

			if operation == "IGNORE" {
				continue
			}

			StTransactionsData = append(StTransactionsData, sir_models.Transaction{
				OperationType: operation,
				Data:          payment.Data.Output,
			})
		}

		utils.Info.Printf("[Kiosco-Insert-Mongo][server: %s] Procesando batch #%d, %d [%d/%d]\n", httpAddress, i+1, batchSize, processed, len(paymentsNormalize))
		if err := provider.mongoRepository.SaveBulkModel(batch); err != nil {
			utils.Error.Printf("[Kiosco-Insert-Mongo][server: %s] Error al procesar batch #%d: %v\n", httpAddress, i+1, err)
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
		utils.Info.Printf("[Kiosco-Insert-Mongo][server: %s] Procesado batch #%d", httpAddress, i+1)
	}
	return inserted, ignored, updated
}

type ApiResponse struct {
	Estado string `json:"estado"`
	Codigo string `json:"codigo"`
	Msg    string `json:"msg"`
	Data   struct {
		ApiToken string `json:"api_token"`
	} `json:"data"`
	Error interface{} `json:"error"`
}

// GenerateTokenApi genera el token de la API y devuelve el api_token o un error
func GenerateTokenApi(httpAddress string, email string, password string) (string, error) {
	route := "/api/login"
	params := map[string]string{
		"email":    email,
		"password": password,
	}

	// Convertimos el map a JSON
	jsonData, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("[kiosco] error al serializar JSON: %v", err)
	}

	// Unimos la URL base con la ruta de la API
	uri := httpAddress + route

	// Creamos la petición POST con el JSON en el cuerpo
	req, err := http.NewRequest("POST", uri, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("[kiosco] error al crear la petición: %v", err)
	}

	// Establecemos las cabeceras de la petición
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("verify", "false")
	req.Header.Set("Accept", "application/json")
	// Enviamos la petición
	client := &http.Client{
		Timeout: 1 * time.Minute,
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("[kiosco] error conectando a la petición: %v", err)
	}
	defer resp.Body.Close()
	// Verificamos si la respuesta es correcta
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		errorBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("[kiosco][ReadAll] error deserializando el body de la respuesta: %v", err)
		}
		return "", fmt.Errorf("StatusCode: %d, Body: %s\n", resp.StatusCode, string(errorBody))
	}

	// Leemos la respuesta del servidor
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("[kiosco] error al leer el body de la respuesta: %v", err)
	}

	// Ahora parseamos la respuesta JSON
	var apiResponse ApiResponse
	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		return "", fmt.Errorf("[kiosco] error al parsear la respuesta JSON: %v", err)
	}

	// Comprobamos si el estado es OK y obtenemos el api_token
	if apiResponse.Estado == "OK" {
		return apiResponse.Data.ApiToken, nil
	} else {
		return "", fmt.Errorf("error en la respuesta: %s", apiResponse.Msg)
	}
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

func createHttpAddress(direccion, puerto string) string {
	var httpAddress = "http://" + direccion
	if !utils.IsEmptyString(puerto) {
		httpAddress = httpAddress + ":" + puerto
	}
	return httpAddress
}

func formatearTipoTarjeta(strGrupoTarjeta string) string {
	// Convertir la cadena a mayúsculas y eliminar espacios
	grupoTarjeta := strings.ToUpper(strings.TrimSpace(strGrupoTarjeta))
	var tipoTarjeta string

	// Usar switch para mapear los valores
	switch grupoTarjeta {
	case "DEBITO":
		tipoTarjeta = "DEBI"
	case "DINERS CLUB":
		tipoTarjeta = "DINE"
	case "MASTERCARD":
		tipoTarjeta = "MAST"
	case "DISCOVER":
		tipoTarjeta = "DISC"
	case "AMERICAN EXPRESS":
		tipoTarjeta = "AMEX"
	case "ALIA":
		tipoTarjeta = "CUOT"
	case "UNION PAY":
		tipoTarjeta = "UPAY"
	default:
		// Si no coincide con ningún caso, devuelve el grupo original
		tipoTarjeta = grupoTarjeta
	}
	return tipoTarjeta
}

func formatHora(hora string) string {
	// Verificar si la cadena está vacía
	if strings.TrimSpace(hora) == "" {
		fmt.Println("Error: la hora está vacía")
		return ""
	}

	horaTx1 := strings.Split(hora, ".")
	horaSinDosPuntos := strings.Replace(horaTx1[0], ":", "", -1)

	return horaSinDosPuntos
}
