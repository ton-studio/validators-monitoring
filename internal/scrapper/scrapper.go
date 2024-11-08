package scrapper

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
	. "validators-health/internal/models"
	"validators-health/internal/notifier"
	"validators-health/internal/services"
)

type ValidatorStatusInfo struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

type Scrapper struct {
	ClickhouseService *services.ClickhouseService
	CacheService      *services.CacheService
	Notifier          *notifier.Notifier
}

func init() {
	log.SetOutput(os.Stdout)
	log.SetPrefix("SCRAPPER: ")
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

func NewScrapper() (*Scrapper, error) {
	clickhouseService, err := services.NewClickhouseService()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ClickHouse service: %w", err)
	}

	cacheService, err := services.NewCacheService()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Cache service: %w", err)
	}

	n, err := notifier.NewNotifier(clickhouseService, cacheService)

	return &Scrapper{
		ClickhouseService: clickhouseService,
		CacheService:      cacheService,
		Notifier:          n,
	}, nil
}

func (s *Scrapper) logHTTPRequest(req *http.Request) {
	log.Printf("HTTP Request: %s %s", req.Method, req.URL.String())
	for header, values := range req.Header {
		for _, value := range values {
			log.Printf("Header: %s: %s", header, value)
		}
	}
}

func (s *Scrapper) logHTTPResponse(resp *http.Response, body []byte) {
	//log.Printf("HTTP Response: %d %s", resp.StatusCode, resp.Status)
	//log.Printf("Response Body: %s", string(body))
}

func (s *Scrapper) GetCycles(cycleID *int) ([]Cycle, error) {
	baseURL := os.Getenv("CYCLE_API_URL")
	req, err := http.NewRequest("GET", baseURL, nil)
	if err != nil {
		return nil, err
	}

	query := req.URL.Query()
	if cycleID != nil {
		query.Add("cycle_id", strconv.Itoa(*cycleID))
	}
	req.URL.RawQuery = query.Encode()

	s.logHTTPRequest(req)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	s.logHTTPResponse(resp, body)

	var cycles []Cycle
	err = json.Unmarshal(body, &cycles)
	if err != nil {
		return nil, err
	}

	return cycles, nil
}

func (s *Scrapper) GetCycleScoreboard(cycleID int, fromTs int, toTs int) ([]CycleScoreboardRow, error) {
	baseURL := os.Getenv("SCOREBOARD_API_URL")
	req, err := http.NewRequest("GET", baseURL, nil)
	if err != nil {
		return nil, err
	}

	query := req.URL.Query()
	query.Add("cycle_id", strconv.Itoa(cycleID))
	if fromTs != 0 && toTs != 0 {
		query.Add("from_ts", strconv.Itoa(fromTs))
		query.Add("to_ts", strconv.Itoa(toTs))
	}

	req.URL.RawQuery = query.Encode()
	s.logHTTPRequest(req)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	s.logHTTPResponse(resp, body)

	var scoreboard ScoreboardResponse
	err = json.Unmarshal(body, &scoreboard)
	if err != nil {
		return nil, err
	}

	return scoreboard.Scoreboard, nil
}

func (s *Scrapper) generateAlertID() (int64, error) {
	alertID, err := s.CacheService.IncrementCounter("alert_id")
	if err != nil {
		return 0, fmt.Errorf("failed to generate alert ID: %w", err)
	}
	return alertID, nil
}

func (s *Scrapper) checkStatusChange(ADNLAddr string, validatorADNL string, efficiency float64, threshold float64) error {

	var currentStatus ValidatorStatus
	if efficiency < threshold {
		currentStatus = StatusNotOK
	} else {
		currentStatus = StatusOK
	}

	key := fmt.Sprintf("validator_status:%s", validatorADNL)
	var previousStatusInfo ValidatorStatusInfo
	found, err := s.CacheService.GetCachedData(key, &previousStatusInfo)
	if err != nil {
		return fmt.Errorf("failed to get cached data: %w", err)
	}

	if !found {

		previousStatusInfo = ValidatorStatusInfo{
			Status:    string(StatusUnknown),
			Timestamp: time.Now(),
		}
	}

	previousStatus := ValidatorStatus(previousStatusInfo.Status)
	previousTimestamp := previousStatusInfo.Timestamp

	if currentStatus != previousStatus {
		duration := time.Since(previousTimestamp)
		newStatusInfo := ValidatorStatusInfo{
			Status:    string(currentStatus),
			Timestamp: time.Now(),
		}
		err = s.CacheService.CacheData(key, newStatusInfo, 0)
		if err != nil {
			return fmt.Errorf("failed to cache new status: %w", err)
		}

		alertID, err := s.generateAlertID()
		if err != nil {
			return fmt.Errorf("failed to generate alert ID: %w", err)
		}

		alert := notifier.Alert{
			ID:                  alertID,
			ADNLAddr:            ADNLAddr,
			ValidatorADNL:       validatorADNL,
			Status:              currentStatus,
			IsAcknowledged:      false,
			LastAlert:           time.Now(),
			Efficiency:          efficiency,
			PreviousStatus:      string(previousStatus),
			PreviousStatusSince: previousTimestamp,
			Duration:            duration,
			Timestamp:           uint32(time.Now().Unix()),
		}

		err = s.Notifier.PublishAlert(alert)
		if err != nil {
			log.Printf("Failed to publish to Redis: %v", err)
		} else {
			log.Printf("Successfully published to validator_notifications")
		}
		log.Printf("Status change detected for ADNL %s: %s -> %s", validatorADNL, previousStatus, currentStatus)

		err = s.ClickhouseService.InsertStatusChange(ADNLAddr, validatorADNL, currentStatus, time.Now())
		if err != nil {
			log.Printf("Failed to insert status change into ClickHouse: %v", err)
		}
	}

	return nil
}

func (s *Scrapper) SaveToClickhouse(scoreboard []CycleScoreboardRow, timeStamp int64) {
	err := s.ClickhouseService.InsertScoreboard(scoreboard, timeStamp)
	if err != nil {
		log.Printf("Error inserting data into ClickHouse: %v", err)
		return
	}

	log.Println("Data successfully saved to ClickHouse.")
}

func (s *Scrapper) ProcessCycles(stop <-chan struct{}, threshold float64, cycleId *int, fromTs int, toTs int, isMigrate bool) error {
	cycles, err := s.GetCycles(cycleId)
	if err != nil {
		log.Printf("Failed to get cycles: %v", err)
		time.Sleep(1 * time.Minute)
		return err
	}

	if err := s.ClickhouseService.InsertCycles(cycles); err != nil {
		log.Printf("Failed to insert cycles: %v", err)
	}

	if err := s.ClickhouseService.InsertCyclesInfo(cycles); err != nil {
		log.Printf("Failed to insert cycles info: %v", err)
	}

	if err := s.ClickhouseService.InsertValidators(cycles); err != nil {
		log.Printf("Failed to insert validators: %v", err)
	}

	var wg sync.WaitGroup
	for _, cycle := range cycles {
		wg.Add(1)
		go func(cycle Cycle) {
			defer wg.Done()
			log.Printf("Processing cycle ID: %d", cycle.CycleID)

			scoreboard, err := s.GetCycleScoreboard(cycle.CycleID, fromTs, toTs)
			if err != nil {
				log.Printf("Failed to get scoreboard for cycle %d: %v", cycle.CycleID, err)
				return
			}
			s.SaveToClickhouse(scoreboard, int64(fromTs*1000))

			if !isMigrate {
				for _, row := range scoreboard {
					err := s.checkStatusChange(row.ADNLAddr, row.ValidatorADNL, row.Efficiency, threshold)
					if err != nil {
						log.Printf("Failed to check status change for validator %s: %v", row.ValidatorADNL, err)
						continue
					}
				}
			}

		}(cycle)
		wg.Wait()

		select {
		case <-stop:
			log.Println("Scrapper is stopping...")
			return nil
		default:
			time.Sleep(10 * time.Second)
		}

		if !isMigrate {
			log.Println("Waiting for 1 minute before the next update...")
			time.Sleep(1 * time.Minute)
		} else {
			time.Sleep(1 * time.Second)
		}

	}

	return nil
}
