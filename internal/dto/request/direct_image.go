package request

type DirectImage struct {
	RequestKey      string `json:"request_key"`
	ExpectedVersion int    `json:"expected_version"`
}
