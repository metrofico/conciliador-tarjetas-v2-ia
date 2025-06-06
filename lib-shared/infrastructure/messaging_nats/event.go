package messaging_nats

type (
	Event struct {
		EventType string
		Data      any
	}
)

func NewEvent(eventType string, data any) *Event {
	return &Event{
		EventType: eventType,
		Data:      data,
	}
}
