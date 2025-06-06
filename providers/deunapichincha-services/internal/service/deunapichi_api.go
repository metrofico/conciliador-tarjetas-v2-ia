package service

import (
	"deunapichincha-services/internal/app/repository"
	"deunapichincha-services/internal/config"
	"deunapichincha-services/internal/models"
	"deunapichincha-services/utils"
	"encoding/json"
	"fmt"
	uuid2 "github.com/google/uuid"
	"io/ioutil"
	"lib-shared/infrastructure/messaging_nats"
	lib_mapper "lib-shared/mapper"
	"lib-shared/sir_models"
	utils3 "lib-shared/utils"
	"net/http"
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

func NewApiProvider(mongoRepository *repository.MongoDataRepository, natsManager *messaging_nats.NatsStarter, cfg config.Config, cache *DataCacheRestaurant) *ApiProviderDatafast {
	return &ApiProviderDatafast{
		mongoRepository: mongoRepository,
		natsManager:     natsManager,
		cfg:             cfg,
		cache:           cache,
	}
}

func (provider *ApiProviderDatafast) RetrievePayments() {
	// Obtener datos de configuración

	// Leer la fecha para el filtro

	// Establecer la zona horaria
	location, err := time.LoadLocation(provider.cfg.TimeZone)
	if err != nil {
		utils.Error.Println("Error al cargar la zona horaria: ", err)
	}

	// Obtener la fecha y hora actual en la zona horaria especificada
	now := time.Now().In(location)
	startExecutor := time.Now()
	// Obtener la fecha de "ayer"
	// Inicializar variables
	limit := 250         // Cantidad de elementos por página
	page := 1            // Página inicial
	hasMorePages := true // Control de paginaciónp
	createdAt := time.Now()

	for hasMorePages {
		// Construcción de la URL con paginación
		paramsUrl := provider.cfg.DeUnaApiReport
		paramsUrl = strings.Replace(paramsUrl, ":dateValue", now.Format("02-01-2006"), 1)
		paramsUrl = strings.Replace(paramsUrl, ":pageValue", fmt.Sprintf("%d", page), 1)
		paramsUrl = strings.Replace(paramsUrl, ":limitValue", fmt.Sprintf("%d", limit), 1)

		utils.Info.Println(fmt.Sprintf("[deunapichi] Consultando página %d: %s", page, paramsUrl))

		// Crear solicitud HTTP
		req, err := http.NewRequest("GET", paramsUrl, nil)
		if err != nil {
			utils.Error.Println("[deunapichi] Error al crear la solicitud HTTP:", err)
			return
		}

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			utils.Error.Println("[deunapichi] Error en la solicitud HTTP:", err)
			return
		}
		body, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			utils.Error.Println("[deunapichi] Error al leer la respuesta:", err)
			return
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			utils.Error.Println(fmt.Sprintf("[deunapichi] Respuesta con código HTTP %d en página %d", resp.StatusCode, page))
			return
		}
		// Leer y deserializar la respuesta JSON
		var paymentResponse models.PaymentResponse
		err = json.Unmarshal(body, &paymentResponse)
		if err != nil {
			utils.Error.Println("[deunapichi] Error al deserializar JSON:", err)
			return
		}

		// Si la página no tiene datos, detener el bucle
		if paymentResponse.Data == nil || len(*paymentResponse.Data) == 0 {
			utils.Info.Println("[deunapichi] No hay más pagos disponibles, terminando consulta.")
			break
		}

		// **IMPORTANTE**: Resetear paymentsNormalize en cada iteración para que cada página tenga su batch separado.
		paymentsNormalize := make([]lib_mapper.Payment, 0)

		// Procesar pagos de la página actual
		for _, payment := range *paymentResponse.Data {
			merchantId := provider.cache.getMerchantId(payment.Store.Name)
			if utils3.IsEmptyString(merchantId) {
				utils.Error.Printf("[deunapichincha][merchantId] El MerchantId para la autorización '%s', storeName '%s',referencia '%s' está vacia\n", payment.TransferNumber, payment.Store.Name, payment.ReferenceId)
				continue
			}
			transformed := sir_models.StTransactions{
				MerchantId:         merchantId,
				FechaTransaccion:   formatFecha(payment.CreatedAt),
				HoraTransaccion:    fmt.Sprintf("%v", payment.TransactionHour),
				Estado:             "0",
				NumeroLote:         GetBatchTime(payment.CreatedAt),
				FaceValue:          payment.Amount,
				IdGrupoTarjeta:     "D_UNAPICHI",
				IdAdquirente:       "D-UNAPICHI",
				NumeroTarjetaMask:  "XXXXXXXXXXXXXXXX",
				NumeroAutorizacion: fmt.Sprintf("%v", payment.TransferNumber),
				NumeroReferencia:   fmt.Sprintf("%v", payment.ReferenceId),
				TipoTransaccion:    "01",
				ResultadoExterno:   "00",
				TipoSwitch:         2,
				OrigenTransaccion:  11,
				Sistema:            "EXTERNO",
			}

			paymentsNormalize = append(paymentsNormalize, lib_mapper.Payment{
				UniqueId:  transformed.MerchantId + transformed.NumeroAutorizacion + transformed.NumeroLote,
				Provider:  "DEUNA-PICHI",
				CreatedAt: createdAt,
				Data: lib_mapper.PaymentData{
					Input:  payment,
					Output: transformed,
				},
			})
		}
		lenPaymentNormalize := len(paymentsNormalize)
		StTransactionsData := make([]sir_models.Transaction, 0)
		var uniqueIds []string
		for _, payment := range paymentsNormalize {
			uniqueIds = append(uniqueIds, payment.UniqueId)
		}
		findPaymentsHash, _ := provider.mongoRepository.FindPaymentsHash(uniqueIds)
		for _, payment := range paymentsNormalize {
			operation := "INSERT"
			for _, paymentHash := range findPaymentsHash {
				if strings.EqualFold(payment.UniqueId, paymentHash.UniqueId) {
					if strings.EqualFold(payment.Hash, paymentHash.Hash) {
						operation = "IGNORE"
						continue
					}
					operation = "UPDATE"
				}
			}
			if operation == "IGNORE" {
				continue
			}

			StTransactionsData = append(StTransactionsData, sir_models.Transaction{
				OperationType: operation,
				Data:          payment.Data.Output,
			})
		}
		// Procesar y almacenar el batch de la página actual en MongoDB
		utils.Info.Println(fmt.Sprintf("[Deunapichi-Insert-Mongo] Procesando batch de pagina #%d - size %d ", page, lenPaymentNormalize))
		saveErr := provider.mongoRepository.SaveBulkModel(paymentsNormalize)
		if saveErr != nil {
			utils.Error.Println(fmt.Sprintf("[Deunapichi-Insert-Mongo] Error en batch de pagina #%d - size %d ", page, lenPaymentNormalize), saveErr)
		}
		if len(StTransactionsData) > 0 {
			uuid, _ := uuid2.NewV7()
			batchAsBytes, _ := json.Marshal(sir_models.WrapperTransactions{
				Id:           uuid.String(),
				Transactions: StTransactionsData,
			})
			provider.natsManager.EventSender.SendMsgBytes("sir.writer.sttransaction", batchAsBytes)
		} else {
			utils.Info.Println(fmt.Sprintf("[DeunaPichincha-Publish] no hay datos para modificar (Empty) (ya existe o no hay cambios) page #%d\n", page))
		}
		// Verificar si hay más páginas
		if page >= int(paymentResponse.Pages) {
			hasMorePages = false
		} else {
			page++ // Pasar a la siguiente página
		}
	}
	elapsedExecutor := time.Since(startExecutor)
	utils.Info.Println("[deunapichi] pagos completado, la tarea tardó " + utils.FormatDuration(elapsedExecutor))
}

func GetBatchTime(createdAt string) string {
	t, err := time.Parse("02/01/2006", createdAt)
	if err != nil {
		fmt.Println("error al parsear la fecha: %w", err)
	}
	fechaFormateada := t.Format("020106")
	return fechaFormateada
}

func formatFecha(fecha string) string {
	parsedDate, err := time.Parse("02/01/2006", fecha)
	if err != nil {
		fmt.Println("Error parsing date:", err)
	}

	// Formatear la fecha en "Y-m-d"
	return parsedDate.Format("2006-01-02")
}
