package request

type FinalRegeneration struct {
	RequestKey string `json:"request_key"`
	ImageIndex int    `json:"image_index"`
}
