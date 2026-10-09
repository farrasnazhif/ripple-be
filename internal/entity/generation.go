package entity

import "time"

type GenerationJob struct {
	SubmissionRejected bool   `json:"-"`
	ID                 string `json:"id"`
	BriefVersion       int    `json:"brief_version"`
	Accepted           bool   `json:"accepted"`
	ShotID             string `json:"shot_id"`
	RequestKey         string `json:"request_key"`
	Kind               string `json:"kind"`
	Prompt             string `json:"prompt"`
	ProviderID         string `json:"provider_id"`
	Status             string `json:"status"`
	// shortcut: outputs use provider URLs, archive media in R2 before relying on permanent history.
	Outputs   []string  `json:"outputs"`
	CreatedAt time.Time `json:"created_at"`
}
