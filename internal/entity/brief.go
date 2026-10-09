package entity

type Brief struct {
	ID      string  `json:"id"`
	RawText string  `json:"raw_text"`
	Version int     `json:"version"`
	Summary *string `json:"summary,omitempty"`
}

type BriefAttachment struct {
	ID          string `json:"id"`
	BriefID     string `json:"brief_id"`
	ObjectKey   string `json:"-"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	Status      string `json:"status"`
}
