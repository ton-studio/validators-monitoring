package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"
	"validators-health/internal/models"
	"validators-health/internal/services"
)

type ValidatorsHandler struct {
	ClickhouseService *services.ClickhouseService
	CacheService      *services.CacheService
}

func NewValidatorsHandler(clickhouseService *services.ClickhouseService, cacheService *services.CacheService) *ValidatorsHandler {
	return &ValidatorsHandler{
		ClickhouseService: clickhouseService,
		CacheService:      cacheService,
	}
}

func (h *ValidatorsHandler) ValidatorStatusesHandler(w http.ResponseWriter, r *http.Request) {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	cycleIDParsed, _ := (strconv.ParseUint(r.URL.Query().Get("cycle_id"), 10, 32))
	cycleID := uint32(cycleIDParsed)

	if fromStr == "" || toStr == "" {
		http.Error(w, "Required params: 'from'  'to'", http.StatusBadRequest)
		return
	}

	fromUnix, err := strconv.ParseInt(fromStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid param 'from'", http.StatusBadRequest)
		return
	}
	toUnix, err := strconv.ParseInt(toStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid param 'to'", http.StatusBadRequest)
		return
	}

	from := time.Unix(fromUnix, 0)
	to := time.Unix(toUnix, 0)

	statuses, err := h.ClickhouseService.GetValidatorsStatuses(from, to, cycleID, h.CacheService)
	meta, err := h.ClickhouseService.GetValidatorsMeta(from, to, cycleID, h.CacheService)

	if err != nil {
		http.Error(w, "Couldn't get validators statuses", http.StatusInternalServerError)
		log.Fatal(err)
		return
	}

	response := struct {
		Statuses map[string]map[uint32]float64 `json:"statuses"`
		Meta     models.Meta                   `json:"meta"`
	}{
		Statuses: statuses,
		Meta:     *meta,
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, "Couldn't encode response", http.StatusInternalServerError)
		return
	}
}
