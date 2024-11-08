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

func NewChartHandler(clickhouseService *services.ClickhouseService, cacheService *services.CacheService) *ChartHandler {
	return &ChartHandler{
		ClickhouseService: clickhouseService,
		CacheService:      cacheService,
	}
}

type ChartHandler struct {
	ClickhouseService *services.ClickhouseService
	CacheService      *services.CacheService
}

type ChartResponse struct {
	ADNL       string                          `json:"adnl"`
	Efficiency []models.EfficiencyDataResponse `json:"efficiency"`
}

func (h *ChartHandler) GetChartData(w http.ResponseWriter, r *http.Request) {
	adnls := r.URL.Query()["adnl"]
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	var fromTime, toTime time.Time
	if from == "" || to == "" {
		toTime = time.Now()
		fromTime = toTime.Add(-24 * time.Hour)
	} else {

		fromTimestamp, err := strconv.ParseInt(from, 10, 64)
		if err != nil {
			http.Error(w, "Invalid 'from' timestamp", http.StatusBadRequest)
			log.Printf("Invalid 'from' timestamp: %v", err)
			return
		}
		toTimestamp, err := strconv.ParseInt(to, 10, 64)
		if err != nil {
			http.Error(w, "Invalid 'to' timestamp", http.StatusBadRequest)
			log.Printf("Invalid 'to' timestamp: %v", err)
			return
		}

		fromTime = time.Unix(fromTimestamp, 0)
		toTime = time.Unix(toTimestamp, 0)
	}

	log.Printf("Handling request with ADNLs: %v, from: %d, to: %d", adnls, fromTime.Unix(), toTime.Unix())

	var results []ChartResponse

	for _, adnl := range adnls {
		cacheKey := adnl + ":" + strconv.FormatInt(fromTime.Unix(), 10) + ":" + strconv.FormatInt(toTime.Unix(), 10)
		var cachedData []models.ValidatorEfficiency

		found, err := h.CacheService.GetCachedData(cacheKey, &cachedData)
		if err != nil {
			http.Error(w, "Failed to access cache", http.StatusInternalServerError)
			log.Printf("Failed to access cache for key %s: %v", cacheKey, err)
			return
		}
		if found {
			log.Printf("Cache hit for key: %s", cacheKey)

			efficiencyList := make([]models.EfficiencyDataResponse, len(cachedData))
			for i, eff := range cachedData {
				efficiencyList[i] = models.EfficiencyDataResponse{
					Timestamp: eff.IntervalStart,
					Value:     eff.Efficiency,
					CycleID:   eff.CycleID,
				}
			}
			results = append(results, ChartResponse{
				ADNL:       adnl,
				Efficiency: efficiencyList,
			})
			continue
		}

		data, err := h.ClickhouseService.GetEfficiencyChartDataCached(adnl, fromTime, toTime, h.CacheService)
		if err != nil {
			http.Error(w, "Failed to query ClickHouse", http.StatusInternalServerError)
			log.Printf("Failed to query ClickHouse for ADNL %s: %v", adnl, err)
			return
		}

		efficiencyList := make([]models.EfficiencyDataResponse, len(data))
		for i, eff := range data {
			efficiencyList[i] = models.EfficiencyDataResponse{
				Timestamp: eff.IntervalStart,
				Value:     eff.Efficiency,
				CycleID:   eff.CycleID,
			}
		}

		err = h.CacheService.CacheData(cacheKey, data, time.Hour)
		if err != nil {
			http.Error(w, "Failed to cache data", http.StatusInternalServerError)
			log.Printf("Failed to cache data for key %s: %v", cacheKey, err)
			return
		}

		results = append(results, ChartResponse{
			ADNL:       adnl,
			Efficiency: efficiencyList,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(results); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		log.Printf("Failed to encode response: %v", err)
	}
}
