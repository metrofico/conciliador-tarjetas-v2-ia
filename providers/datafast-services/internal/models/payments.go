package models

import "encoding/xml"

type PaymentData struct {
	FechaTrx       string  `xml:"FECHA_TRX"`
	MID            string  `xml:"MID"`
	NomComercio    string  `xml:"NOM_COMERCIO"`
	TID            string  `xml:"TID"`
	Lote           string  `xml:"LOTE"`
	Referencia     string  `xml:"REFERENCIA"`
	Bin            string  `xml:"BIN"`
	CodCredito     string  `xml:"COD_CREDITO"`
	NomCredito     string  `xml:"NOM_CREDITO"`
	Cuotas         string  `xml:"CUOTAS"`
	Autorizacion   string  `xml:"AUTORIZACION"`
	Adquirente     string  `xml:"ADQUIRENTE"`
	VBase0         float64 `xml:"V_BASE_0"`
	VBaseImponible float64 `xml:"V_BASE_IMPONIBLE"`
	VIva           float64 `xml:"V_IVA"`
	VInteres       float64 `xml:"V_INTERES"`
	VTotal         float64 `xml:"V_TOTAL"`
}
type PaymentResponse struct {
	XMLName xml.Name       `xml:"TRX"`
	Records *[]PaymentData `xml:"R"`
}
