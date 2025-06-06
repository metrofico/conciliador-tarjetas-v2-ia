package models

type PaymentData struct {
	MerchantId         string `json:"Merchantid"          bson:"Merchantid"`
	FechaTransaccion   string `json:"Fecha_Transaccion"   bson:"Fecha_Transaccion"`
	HoraTransaccion    string `json:"Hora_Transaccion"    bson:"Hora_Transaccion"`
	Estado             string `json:"Estado"              bson:"Estado"`
	NumeroLote         string `json:"Numero_Lote"         bson:"Numero_Lote"`
	FaceValue          string `json:"Face_Value"          bson:"Face_Value"`
	IdGrupoTarjeta     string `json:"Id_Grupo_Tarjeta"    bson:"Id_Grupo_Tarjeta"`
	IdAdquirente       string `json:"Id_Adquirente"       bson:"Id_Adquirente"`
	NumeroTarjetaMask  string `json:"numero_tarjeta_mask" bson:"numero_tarjeta_mask"`
	NumeroAutorizacion string `json:"Numero_Autorizacion" bson:"Numero_Autorizacion"`
	NumeroReferencia   string `json:"Numero_Referencia"   bson:"Numero_Referencia"`
	TipoTransaccion    string `json:"Tipo_Transaccion"    bson:"Tipo_Transaccion"`
	ResultadoExterno   string `json:"Resultado_externo"   bson:"Resultado_externo"`
	TipoSwitch         string `json:"Tipo_Switch"         bson:"Tipo_Switch"`
	OrigenTransaccion  string `json:"Origen_Transaccion"  bson:"Origen_Transaccion"`
}
