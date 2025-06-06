package mapper

import (
	"lib-shared/sir_models"
	"time"
)

type PaymentData struct {
	Input  interface{}               `json:"input"  bson:"input"`
	Output sir_models.StTransactions `json:"output" bson:"output"`
}
type Payment struct {
	UniqueId      string      `json:"uniqueId" bson:"uniqueId"`
	Hash          string      `json:"hash" bson:"hash"`
	StoreId       string      `json:"storeId" bson:"storeId"`
	ConciliatorId string      `json:"conciliatorId" bson:"conciliatorId"`
	Provider      string      `json:"provider"  bson:"provider"`
	CreatedAt     time.Time   `json:"createdAt"  bson:"createdAt"`
	Data          PaymentData `json:"data"  bson:"data"`
}
