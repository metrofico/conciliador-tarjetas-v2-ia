package sir_models

import (
	"crypto/sha3"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

type WrapperTransactions struct {
	Id           string
	Transactions []Transaction
}
type Transaction struct {
	OperationType string // UPDATE, INSERT, DELETE
	Data          StTransactions
}
type StTransactions struct {
	MerchantId         string  `json:"merchantId"            bson:"merchantId"`
	FechaTransaccion   string  `json:"fecha_Transaccion"     bson:"fecha_Transaccion"`
	HoraTransaccion    string  `json:"hora_Transaccion"      bson:"hora_Transaccion"`
	Estado             string  `json:"estado"                bson:"estado"`
	NumeroLote         string  `json:"numero_Lote"           bson:"numero_Lote"`
	FaceValue          string  `json:"face_Value"            bson:"face_Value"`
	IdGrupoTarjeta     string  `json:"id_Grupo_Tarjeta"      bson:"id_Grupo_Tarjeta"`
	IdAdquirente       string  `json:"id_Adquirente"         bson:"id_Adquirente"`
	NumeroTarjetaMask  string  `json:"numero_Tarjeta_Mask"   bson:"numero_Tarjeta_Mask"`
	NumeroAutorizacion string  `json:"numero_Autorizacion"   bson:"numero_Autorizacion"`
	NumeroReferencia   string  `json:"numero_Referencia"     bson:"numero_Referencia"`
	TipoTransaccion    string  `json:"tipo_Transaccion"      bson:"tipo_Transaccion"`
	ResultadoExterno   string  `json:"resultado_Externo"     bson:"resultado_Externo"`
	TipoSwitch         int     `json:"tipo_Switch"           bson:"tipo_Switch"`
	OrigenTransaccion  int     `json:"origen_Transaccion"    bson:"origen_Transaccion"`
	Sistema            string  `json:"sistema"               bson:"sistema"`
	Voucher            string  `json:"voucher"               bson:"voucher"`
	CuentaNombre       string  `json:"cuentaNombre"          bson:"cuentaNombre"`
	Subtotal           float32 `json:"subtotal"              bson:"subtotal"`
	Descuento          float32 `json:"descuento"             bson:"descuento"`
	Iva                float32 `json:"iva"                   bson:"iva"`
	IvaAplicado        float32 `json:"ivaAplicado"           bson:"ivaAplicado"`
	FidelizacionOpera  float32 `json:"fidelizacionOpera"     bson:"fidelizacionOpera"`
	FidelizacionMerca  float32 `json:"fidelizacionMerca"     bson:"fidelizacionMerca"`
	FidelizacionTotal  float32 `json:"fidelizacionTotal"     bson:"fidelizacionTotal"`
	FidelizacionValor  float32 `json:"fidelizacionValor"     bson:"fidelizacionValor"`
}

func (transaction StTransactions) GetUniqueId() string {
	var sb strings.Builder
	sb.WriteString(transaction.MerchantId)
	sb.WriteString(transaction.FechaTransaccion)
	sb.WriteString(transaction.HoraTransaccion)
	sb.WriteString(transaction.NumeroReferencia)
	sb.WriteString(transaction.NumeroAutorizacion)
	sb.WriteString(transaction.NumeroTarjetaMask)
	hash := sha3.New256()
	hash.Write([]byte(sb.String()))
	// Devolver el hash como una cadena hex
	return hex.EncodeToString(hash.Sum(nil))
}
func (transaction StTransactions) GetTransactionHash() string {
	// Concatenar los valores de los campos
	var sb strings.Builder
	sb.WriteString(transaction.MerchantId)
	sb.WriteString(transaction.FechaTransaccion)
	sb.WriteString(transaction.HoraTransaccion)
	sb.WriteString(transaction.Estado)
	sb.WriteString(transaction.NumeroLote)
	sb.WriteString(transaction.FaceValue)
	sb.WriteString(transaction.IdGrupoTarjeta)
	sb.WriteString(transaction.IdAdquirente)
	sb.WriteString(transaction.NumeroTarjetaMask)
	sb.WriteString(transaction.NumeroAutorizacion)
	sb.WriteString(transaction.NumeroReferencia)
	sb.WriteString(transaction.TipoTransaccion)
	sb.WriteString(transaction.ResultadoExterno)
	sb.WriteString(strconv.Itoa(transaction.TipoSwitch))        // Convierte los enteros
	sb.WriteString(strconv.Itoa(transaction.OrigenTransaccion)) // Convierte los enteros
	sb.WriteString(transaction.Sistema)
	sb.WriteString(transaction.Voucher)
	sb.WriteString(transaction.CuentaNombre)
	sb.WriteString(fmt.Sprintf("%f", transaction.Subtotal))    // Convierte los float32
	sb.WriteString(fmt.Sprintf("%f", transaction.Descuento))   // Convierte los float32
	sb.WriteString(fmt.Sprintf("%f", transaction.Iva))         // Convierte los float32
	sb.WriteString(fmt.Sprintf("%f", transaction.IvaAplicado)) // Convierte los float32
	sb.WriteString(fmt.Sprintf("%f", transaction.FidelizacionOpera))
	sb.WriteString(fmt.Sprintf("%f", transaction.FidelizacionMerca))
	sb.WriteString(fmt.Sprintf("%f", transaction.FidelizacionTotal))
	sb.WriteString(fmt.Sprintf("%f", transaction.FidelizacionValor))

	// Convertir la cadena resultante a un hash SHA256
	hash := sha3.New256()
	hash.Write([]byte(sb.String()))
	// Devolver el hash como una cadena hex
	return hex.EncodeToString(hash.Sum(nil))
}
