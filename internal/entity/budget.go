package entity

import "time"

// GenerationRates are conservative USD-cent estimates per provider request, not invoices.
type GenerationRates map[string]int64

type GenerationUsage struct {
	OperationID   string    `json:"operation_id"`
	Operation     string    `json:"operation"`
	Description   string    `json:"description"`
	Actor         string    `json:"actor"`
	EstimateCents int64     `json:"estimate_cents"`
	State         string    `json:"state"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

type ProjectBudget struct {
	LimitCents     *int64            `json:"limit_cents"`
	ReservedCents  int64             `json:"reserved_cents"`
	SpentCents     int64             `json:"spent_cents"`
	AvailableCents int64             `json:"available_cents"`
	Rates          GenerationRates   `json:"rates"`
	Usage          []GenerationUsage `json:"usage"`
}
