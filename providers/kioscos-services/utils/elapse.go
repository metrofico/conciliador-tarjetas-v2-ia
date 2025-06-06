package utils

import (
	"fmt"
	"time"
)

func FormatDuration(d time.Duration) string {
	// Convertimos la duración en minutos y segundos
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	milliseconds := int(d.Milliseconds()) % 1000

	switch {
	case minutes > 0 && seconds > 0:
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	case minutes > 0:
		return fmt.Sprintf("%dm", minutes)
	case seconds > 0:
		return fmt.Sprintf("%ds", seconds)
	default:
		return fmt.Sprintf("%dms", milliseconds)
	}
}
