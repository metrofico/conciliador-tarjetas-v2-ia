package service

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/nats-io/nats.go/jetstream"
	"io"
	"lib-shared/infrastructure/messaging_nats"
	lib_mapper "lib-shared/mapper"
	"lib-shared/sir_models"
	"net/http"
	db "sir-writer/internal/app/databases"
	"sir-writer/internal/app/repository"
	"sir-writer/internal/config"
	"sir-writer/utils"
	"strings"
	"sync"
	"time"
)

type ApiProviderDatafast struct {
	mongoRepository *repository.MongoDataRepository
	country         string
	cfg             config.Config
	natsManager     *messaging_nats.NatsStarter
	cache           *DataCacheRestaurant
}

type IpAddressRestaurant struct {
	Direccion   string
	Puerto      string
	TokenAcceso string
	Email       string
	Clave       string
}

func NewApiProvider(mongoRepository *repository.MongoDataRepository, natsManager *messaging_nats.NatsStarter, cfg config.Config, cache *DataCacheRestaurant) *ApiProviderDatafast {
	return &ApiProviderDatafast{
		mongoRepository: mongoRepository,
		natsManager:     natsManager,
		cfg:             cfg,
		cache:           cache,
	}
}

func (provider *ApiProviderDatafast) SavePaymentsTransactions(incomingMessage sir_models.WrapperTransactions, msg jetstream.Msg) {
	wrapper := incomingMessage.Transactions
	utils.Info.Printf("[sql-sir][StTransactions] starting batch %s with payments %d\n", incomingMessage.Id, len(wrapper))
	const batchSize = 25
	numWorkers := 10
	// Canal para repartir el trabajo
	batchChan := make(chan []sir_models.Transaction, numWorkers)
	var wg sync.WaitGroup
	// Lanza los workers para procesar las transacciones
	inserted := 0
	updated := 0
	deleted := 0
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer func() {
				msg.InProgress()
				wg.Done()
			}()
			for batch := range batchChan {
				insertedResult, updatedResult, deletedResult := provider.processBatch(batch)
				inserted += insertedResult
				updated += updatedResult
				deleted += deletedResult
				msg.InProgress()
				utils.Info.Printf("[sql-sir][StTransactions] end batch insert:%d, updated:%d, deleted:%d\n", insertedResult, updatedResult, deletedResult)
			}
		}()
	}

	// Dividir las transacciones en lotes
	for i := 0; i < len(wrapper); i += batchSize {
		end := i + batchSize
		if end > len(wrapper) {
			end = len(wrapper)
		}
		batchChan <- wrapper[i:end]
	}
	close(batchChan)
	wg.Wait()
	utils.Info.Printf("[sql-sir][StTransactions] all batched finished inserted %d, updated %d, deleted %d\n", inserted, updated, deleted)
}
func (provider *ApiProviderDatafast) processBatch(batch []sir_models.Transaction) (int, int, int) {
	conn, err := db.NewSQLServerConnection(provider.cfg.SqlServerSir.JDBC)
	if err != nil {
		utils.Error.Printf("Error conectando a la base de datos: %v\n", err)
		return 0, 0, 0
	}
	defer conn.Close()

	// Iterar sobre cada transacción en el batch
	inserted := 0
	updated := 0
	deleted := 0
	for _, wrapper := range batch {
		switch wrapper.OperationType {
		case "INSERT":
			provider.insertTransaction(conn, wrapper.Data)
			inserted++
			continue
		case "UPDATE":
			provider.updateTransaction(conn, wrapper.Data)
			updated++
			continue
		case "DELETE":
			provider.deleteTransaction(conn, wrapper.Data)
			deleted++
			continue
		default:
			utils.Error.Printf("Operación desconocida: %s\n", wrapper.OperationType)
		}
	}
	return inserted, updated, deleted
}
func (provider *ApiProviderDatafast) insertTransaction(conn *db.SQLServerConnection, data sir_models.StTransactions) {
	ex, err := conn.Exec(`
		INSERT INTO ST_Transaccional (
			Merchantid, Fecha_Transaccion, Hora_Transaccion, Estado,
			Numero_Lote, Face_Value, Id_Grupo_Tarjeta, Id_Adquirente,
			numero_tarjeta_mask, Numero_Autorizacion, Numero_Referencia,
			Tipo_Transaccion, Resultado_Externo, Tipo_Switch, origen_Transaccion,
			Sistema, Voucher, CuentaNombre, Subtotal, Descuento, Iva,
			IvaAplicado, FidelizacionOpera, FidelizacionMerca, FidelizacionTotal,
			FidelizacionValor
		) VALUES (
			@merchantId, @fechaTransaccion, @horaTransaccion, @estado,
			@numeroLote, @faceValue, @idGrupoTarjeta, @idAdquirente,
			@numeroTarjetaMask, @numeroAutorizacion, @numeroReferencia,
			@tipoTransaccion, @resultadoExterno, @tipoSwitch, @origenTransaccion,
			@sistema, @voucher, @cuentaNombre, @subtotal, @descuento, @iva,
			@ivaAplicado, @fidelizacionOpera, @fidelizacionMerca, @fidelizacionTotal,
			@fidelizacionValor
		)`,
		sql.Named("merchantId", data.MerchantId),
		sql.Named("fechaTransaccion", data.FechaTransaccion),
		sql.Named("horaTransaccion", data.HoraTransaccion),
		sql.Named("estado", data.Estado),
		sql.Named("numeroLote", data.NumeroLote),
		sql.Named("faceValue", data.FaceValue),
		sql.Named("idGrupoTarjeta", data.IdGrupoTarjeta),
		sql.Named("idAdquirente", data.IdAdquirente),
		sql.Named("numeroTarjetaMask", data.NumeroTarjetaMask),
		sql.Named("numeroAutorizacion", data.NumeroAutorizacion),
		sql.Named("numeroReferencia", data.NumeroReferencia),
		sql.Named("tipoTransaccion", data.TipoTransaccion),
		sql.Named("resultadoExterno", data.ResultadoExterno),
		sql.Named("tipoSwitch", data.TipoSwitch),
		sql.Named("origenTransaccion", data.OrigenTransaccion),
		sql.Named("sistema", data.Sistema),
		sql.Named("voucher", data.Voucher),
		sql.Named("cuentaNombre", data.CuentaNombre),
		sql.Named("subtotal", data.Subtotal),
		sql.Named("descuento", data.Descuento),
		sql.Named("iva", data.Iva),
		sql.Named("ivaAplicado", data.IvaAplicado),
		sql.Named("fidelizacionOpera", data.FidelizacionOpera),
		sql.Named("fidelizacionMerca", data.FidelizacionMerca),
		sql.Named("fidelizacionTotal", data.FidelizacionTotal),
		sql.Named("fidelizacionValor", data.FidelizacionValor),
	)
	if err != nil {
		utils.Error.Printf("Error al insertar: %v\n", err)
	}
	_, err = ex.RowsAffected()
	if err != nil {
		utils.Error.Printf("Error al retornar RowsAffected: %v\n", err)
	}
	//utils.Info.Printf("[sql-sir][Insert] merchant id: %s\n", data.MerchantId)
	//utils.Info.Printf("[sql-sir][Insert] filas afectadas en el insert: %d\n", affected)
}
func (provider *ApiProviderDatafast) updateTransaction(conn *db.SQLServerConnection, data sir_models.StTransactions) {
	_, err := conn.Exec(`
		UPDATE ST_Transaccional SET
			Hora_Transaccion = @horaTransaccion,
			Estado = @estado,
			Numero_Lote = @numeroLote,
			Face_Value = @faceValue,
			Id_Grupo_Tarjeta = @idGrupoTarjeta,
			Id_Adquirente = @idAdquirente,
			numero_tarjeta_mask = @numeroTarjetaMask,
			Numero_Autorizacion = @numeroAutorizacion,
			Numero_Referencia = @numeroReferencia,
			Tipo_Transaccion = @tipoTransaccion,
			Resultado_Externo = @resultadoExterno,
			Tipo_Switch = @tipoSwitch,
			origen_Transaccion = @origenTransaccion,
			Sistema = @sistema,
			Voucher = @voucher,
			CuentaNombre = @cuentaNombre,
			Subtotal = @subtotal,
			Descuento = @descuento,
			Iva = @iva,
			IvaAplicado = @ivaAplicado,
			FidelizacionOpera = @fidelizacionOpera,
			FidelizacionMerca = @fidelizacionMerca,
			FidelizacionTotal = @fidelizacionTotal,
			FidelizacionValor = @fidelizacionValor
		WHERE
			Merchantid = @merchantId AND Fecha_Transaccion = @fechaTransaccion`,
		sql.Named("merchantId", data.MerchantId),
		sql.Named("fechaTransaccion", data.FechaTransaccion),
		sql.Named("horaTransaccion", data.HoraTransaccion),
		sql.Named("estado", data.Estado),
		sql.Named("numeroLote", data.NumeroLote),
		sql.Named("faceValue", data.FaceValue),
		sql.Named("idGrupoTarjeta", data.IdGrupoTarjeta),
		sql.Named("idAdquirente", data.IdAdquirente),
		sql.Named("numeroTarjetaMask", data.NumeroTarjetaMask),
		sql.Named("numeroAutorizacion", data.NumeroAutorizacion),
		sql.Named("numeroReferencia", data.NumeroReferencia),
		sql.Named("tipoTransaccion", data.TipoTransaccion),
		sql.Named("resultadoExterno", data.ResultadoExterno),
		sql.Named("tipoSwitch", data.TipoSwitch),
		sql.Named("origenTransaccion", data.OrigenTransaccion),
		sql.Named("sistema", data.Sistema),
		sql.Named("voucher", data.Voucher),
		sql.Named("cuentaNombre", data.CuentaNombre),
		sql.Named("subtotal", data.Subtotal),
		sql.Named("descuento", data.Descuento),
		sql.Named("iva", data.Iva),
		sql.Named("ivaAplicado", data.IvaAplicado),
		sql.Named("fidelizacionOpera", data.FidelizacionOpera),
		sql.Named("fidelizacionMerca", data.FidelizacionMerca),
		sql.Named("fidelizacionTotal", data.FidelizacionTotal),
		sql.Named("fidelizacionValor", data.FidelizacionValor),
	)
	if err != nil {
		utils.Error.Printf("Error al actualizar: %v\n", err)
	}
}
func (provider *ApiProviderDatafast) deleteTransaction(conn *db.SQLServerConnection, data sir_models.StTransactions) {
	_, err := conn.Exec(`
		DELETE FROM ST_Transaccional WHERE
			Merchantid = @merchantId AND Fecha_Transaccion = @fechaTransaccion`,
		sql.Named("merchantId", data.MerchantId),
		sql.Named("fechaTransaccion", data.FechaTransaccion),
	)
	if err != nil {
		utils.Error.Printf("Error al eliminar: %v\n", err)
	}
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
		utils.Error.Println("[kiosco] error al serializar JSON:", err)
		return "", err
	}

	// Unimos la URL base con la ruta de la API
	uri := httpAddress + route

	// Creamos la petición POST con el JSON en el cuerpo
	req, err := http.NewRequest("POST", uri, bytes.NewBuffer(jsonData))
	if err != nil {
		utils.Error.Println("[kiosco] error al crear la petición:", err)
		return "", err
	}

	// Establecemos las cabeceras de la petición
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("verify", "false")
	req.Header.Set("Accept", "application/json")
	// Enviamos la petición
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		utils.Error.Println("[kiosco] error conectando a la petición:", err)
		return "", err
	}
	defer resp.Body.Close()
	// Verificamos si la respuesta es correcta
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		errorBody, err := io.ReadAll(resp.Body)
		if err != nil {
			utils.Error.Println("[kiosco][ReadAll] error deserializando el body de la respuesta:", err)
		}
		utils.Error.Printf("StatusCode: %d, Body: %s\n", resp.StatusCode, string(errorBody))
		return "", err
	}

	// Leemos la respuesta del servidor
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		utils.Error.Println("[kiosco] error al leer el body de la respuesta:", err)
		return "", err
	}

	// Ahora parseamos la respuesta JSON
	var apiResponse ApiResponse
	err = json.Unmarshal(body, &apiResponse)
	if err != nil {
		utils.Error.Println("[kiosco] error al parsear la respuesta JSON:", err)
		return "", err
	}

	// Comprobamos si el estado es OK y obtenemos el api_token
	if apiResponse.Estado == "OK" {
		return apiResponse.Data.ApiToken, nil
	} else {
		utils.Error.Println("[kiosco] Error en la respuesta: " + apiResponse.Msg)
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
