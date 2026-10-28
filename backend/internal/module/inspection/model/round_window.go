package model

import (
	"time"

	"anxuncloud/internal/pkg/inspectionrule"
)

func RoundWindowEnd(taskDate time.Time, window string) (time.Time, bool) {
	return inspectionrule.RoundWindowEnd(taskDate, window)
}

func ShouldOverdue(taskDate time.Time, timeWindow string, now time.Time) bool {
	return inspectionrule.ShouldOverdue(taskDate, timeWindow, now)
}
