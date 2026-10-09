package entity

import "time"

type Project struct {
	ID          string    `json:"id"`
	CreatedBy   string    `json:"created_by"`
	BriefID     string    `json:"brief_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}
