package handlers

import (
	"net/http"
	"validators-health/internal/services"
)

func NewHandlers(clickhouseService *services.ClickhouseService, cacheService *services.CacheService) *Handlers {
	return &Handlers{
		ClickhouseService: clickhouseService,
		CacheService:      cacheService,
	}
}

type Handlers struct {
	ClickhouseService *services.ClickhouseService
	CacheService      *services.CacheService
}

func (h *Handlers) HealthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "healthy"}`))
}

func (h *Handlers) ChartHandler(w http.ResponseWriter, r *http.Request) {
	chartHandler := NewChartHandler(h.ClickhouseService, h.CacheService)
	chartHandler.GetChartData(w, r)
}

func (h *Handlers) ValidatorStatusesHandler(w http.ResponseWriter, r *http.Request) {
	validatorsHandler := NewValidatorsHandler(h.ClickhouseService, h.CacheService)
	validatorsHandler.ValidatorStatusesHandler(w, r)
}
