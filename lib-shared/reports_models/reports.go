package reports_models

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"
)

type ReportConciliator struct {
	ConciliatorId string         `json:"conciliatorId" bson:"conciliatorId"`
	CreatedAt     time.Time      `json:"created_at" bson:"createdAt"`
	StartedAt     *time.Time     `json:"started_at" bson:"startedAt"`
	CompletedAt   *time.Time     `json:"completed_at" bson:"completedAt"`
	ElapsedTime   int            `json:"elapsed_time" bson:"elapsedTime"`
	Entries       *EntriesReport `json:"entries" bson:"entries"`
	Request       Request        `json:"request" bson:"request"`
}
type EntriesReport struct {
	Inserted uint32 `json:"inserted"`
	Updated  uint32 `json:"updated"`
	Ignored  uint32 `json:"ignored"`
}

// GetCreatedAtFormatted devuelve la fecha de creación formateada en español para la zona horaria "America/Guayaquil".
func (r *ReportConciliator) GetCreatedAtFormatted(timeZone string) string {
	return formatDateInSpanish(r.CreatedAt, timeZone)
}
func (r *ReportConciliator) GetStartedAtFormatted(timeZone string) string {
	if r.StartedAt == nil {
		return "No ha empezado"
	}
	return formatDateInSpanish(*r.StartedAt, timeZone)
}

// GetCompletedAtFormatted devuelve la fecha de finalización formateada en español para la zona horaria "America/Guayaquil".
func (r *ReportConciliator) GetCompletedAtFormatted(timeZone string) string {
	if r.CompletedAt == nil {
		return "No ha finalizado"
	}
	return formatDateInSpanish(*r.CompletedAt, timeZone)
}

// GetElapsedTimeFormatted devuelve el tiempo transcurrido formateado en un formato conciso.
func (r *ReportConciliator) GetElapsedTimeFormatted() string {
	return formatDuration(time.Duration(r.ElapsedTime) * time.Second)
}

// formatDateInSpanish formatea una fecha en español para la zona horaria especificada.
func formatDateInSpanish(t time.Time, timezone string) string {
	loc, _ := time.LoadLocation(timezone)
	if loc != nil {
		t = t.In(loc)
	}
	day := t.Day()
	month := t.Month()
	year := t.Year()
	hour := t.Hour()
	minute := t.Minute()
	second := t.Second()
	months := map[time.Month]string{
		time.January:   "enero",
		time.February:  "febrero",
		time.March:     "marzo",
		time.April:     "abril",
		time.May:       "mayo",
		time.June:      "junio",
		time.July:      "julio",
		time.August:    "agosto",
		time.September: "septiembre",
		time.October:   "octubre",
		time.November:  "noviembre",
		time.December:  "diciembre",
	}
	timeStr := fmt.Sprintf("%02d:%02d:%02d", hour, minute, second)
	return fmt.Sprintf("%d de %s del %d a las %s", day, months[month], year, timeStr)
}

// formatDuration formatea una duración en un formato conciso.
func formatDuration(d time.Duration) string {
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60

	if minutes > 0 && seconds > 0 {
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	} else {
		return fmt.Sprintf("%ds", seconds)
	}
}

type ReportConciliatorJsonResponse struct {
	ConciliatorId string                     `json:"conciliator_id"`
	CreatedAt     string                     `json:"created_at"`
	StartedAt     string                     `json:"started_at"`
	CompletedAt   string                     `json:"completed_at" `
	ElapsedTime   string                     `json:"elapsed_time"`
	Entries       *ReportEntriesJsonResponse `json:"entries"`
	Request       Request                    `json:"request"`
	ReportData    []ReportDataBson           `json:"report_data"`
}
type ReportEntriesJsonResponse struct {
	Inserted uint32 `json:"inserted"`
	Updated  uint32 `json:"updated"`
	Ignored  uint32 `json:"ignored"`
}
type Request struct {
	Service string `json:"service" bson:"service"`
	Date    string `json:"date" bson:"date"`
}

func (receiver Request) Hash() string {
	combined := receiver.Service + receiver.Date
	combined = strings.ToLower(combined)
	combined = strings.ReplaceAll(combined, " ", "")
	hash := sha256.Sum256([]byte(combined))
	hashStr := fmt.Sprintf("%x", hash)
	return hashStr
}

type StartedReport struct {
	ConciliatorId string    `json:"conciliatorId"`
	StartedTime   time.Time `json:"started_time"`
}
type CompletedReport struct {
	ConciliatorId string                 `json:"conciliatorId"`
	CompletedAt   time.Time              `json:"completedAt" `
	Entries       EntriesCompletedReport `json:"entries" `
	ElapsedTime   uint32                 `json:"elapsedTime"`
}
type EntriesCompletedReport struct {
	Inserted uint32 `json:"inserted"`
	Updated  uint32 `json:"updated"`
	Ignored  uint32 `json:"ignored"`
}
type ReportData struct {
	ConciliatorId string    `json:"conciliator_id" bson:"conciliatorId"`
	Type          string    `json:"type" bson:"type"` // ERROR, INFO
	Message       string    `json:"message" bson:"message"`
	Metadata      Metadata  `json:"metadata" bson:"metadata"`
	CreatedAt     time.Time `json:"created_at" bson:"createdAt"`
}
type ReportDataBson struct {
	ConciliatorId string       `json:"conciliator_id" bson:"conciliatorId"`
	Type          string       `json:"type" bson:"type"` // ERROR, INFO
	Message       string       `json:"message" bson:"message"`
	Metadata      MetadataBson `json:"metadata" bson:"metadata"`
	CreatedAt     time.Time    `json:"created_at" bson:"createdAt"`
}
type Metadata struct {
	Content interface{} `json:"content" bson:"content"`
}
type MetadataBson struct {
	Content map[string]interface{} `json:"content" bson:"content"`
}
