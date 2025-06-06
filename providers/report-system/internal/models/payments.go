package models

type PaymentData struct {
	ReferenceId     string `json:"referenceId"             bson:"referenceId"`
	Amount          string `json:"amount"                  bson:"amount"`
	CreatedAt       string `json:"createdAt"               bson:"errorCode"`
	TransactionHour string `json:"transactionHour"         bson:"transactionHour"`
	TransferNumber  string `json:"transferNumber"          bson:"transferNumber"`
	Store           struct {
		Id   int    `json:"id"                      bson:"id"`
		Name string `json:"name"                    bson:"name"`
	} `json:"store"         bson:"store"`
}

type PaymentResponse struct {
	TotalData   int            `json:"totalData"`
	Pages       int64          `json:"pages"`
	CurrentPage int            `json:"currentPage"`
	Data        *[]PaymentData `json:"data"`
}
