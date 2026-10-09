package handler

import (
	"net/http"

	"github.com/farrasnazhif/ripple-be/internal/dto/request"
	"github.com/farrasnazhif/ripple-be/internal/helpers"
	"github.com/farrasnazhif/ripple-be/internal/service"
)

func PreviewImpact(w http.ResponseWriter, r *http.Request) {
	var input request.ImpactPreview
	if err := helpers.ReadJSON(r, &input); err != nil {
		helpers.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	preview, err := service.PreviewImpact(input)
	if err != nil {
		helpers.ErrorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	helpers.SuccessResponse(w, http.StatusOK, "impact preview", preview)
}
