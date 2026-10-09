package handler

import (
	"net/http"

	"github.com/farrasnazhif/ripple-be/internal/helpers"
	"github.com/farrasnazhif/ripple-be/internal/service"
)

// DemoGraph serves only the labeled synthetic fixture; real projects need persistence.
func DemoGraph(w http.ResponseWriter, r *http.Request) {
	scenario := r.URL.Query().Get("scenario")
	if scenario == "" {
		scenario = "s1"
	}
	graph, err := service.DemoGraph(scenario)
	if err != nil {
		helpers.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	helpers.SuccessResponse(w, http.StatusOK, "synthetic Laju demo graph", graph)
}
