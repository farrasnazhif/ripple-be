package request

type ProjectCreate struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
