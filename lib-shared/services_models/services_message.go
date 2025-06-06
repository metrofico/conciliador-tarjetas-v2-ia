package services_models

import "time"

type ServiceMessageDate struct {
	ConciliatorId string    `json:"conciliatorId"`
	ProcessDate   time.Time `json:"process_date"`
	HashId        string    `json:"hashId"`
}
