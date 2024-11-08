package main

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
	"validators-health/internal/clients/clickhouse"
	"validators-health/internal/handlers"
	"validators-health/internal/migrations"
	"validators-health/internal/notifier"
	"validators-health/internal/scrapper"
	"validators-health/internal/services"
)

var (
	clickhouseService *services.ClickhouseService
	cacheService      *services.CacheService
)

func main() {
	if err := migrations.CreateTables(); err != nil {
		log.Fatalf("Error during table creation: %v (%s, %s, %s, %s)",
			err,
			os.Getenv("CLICKHOUSE_HOST"),
			os.Getenv("CLICKHOUSE_DB"),
			os.Getenv("CLICKHOUSE_USER"),
			os.Getenv("CLICKHOUSE_PASSWORD"))
	}

	initServices()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	var wg sync.WaitGroup
	wg.Add(3)

	stopChannel := make(chan struct{})

	go runScrapper(&wg, stopChannel)
	go runNotifier(&wg, stopChannel)
	go runBackend(&wg, stopChannel)

	<-stop
	log.Println("Shutting down gracefully...")

	close(stopChannel)
	wg.Wait()

	<-stop
	log.Println("Shutting down gracefully...")
}

func initServices() {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: os.Getenv("REDIS_PASSWORD"),
	})
	cacheService = &services.CacheService{RedisClient: redisClient}

	clickhouseClient, err := clickhouse.GetClickHouseClient()
	if err != nil {
		log.Fatalf("   ClickHouse : %v", err)
	}
	clickhouseService = &services.ClickhouseService{
		DB: clickhouseClient.DB,
	}
}

func runScrapper(wg *sync.WaitGroup, stop <-chan struct{}) {
	defer wg.Done()
	log.Println("Starting Scrapper...")
	s, _ := scrapper.NewScrapper()
	effThreshold, _ := strconv.Atoi(os.Getenv("EFFICIENCY_THRESHOLD"))
	if os.Getenv("BATCH_SCRAPPING_START_CYCLE_ID") != "" && os.Getenv("BATCH_SCRAPPING_FINISH_CYCLE_ID") != "" {
		startCycleId, _ := strconv.Atoi(os.Getenv("BATCH_SCRAPPING_START_CYCLE_ID"))
		finishCycleId, _ := strconv.Atoi(os.Getenv("BATCH_SCRAPPING_FINISH_CYCLE_ID"))

		log.Printf("Starting BATCH scrapping from %d to %d", startCycleId, finishCycleId)

		if startCycleId > 0 && finishCycleId > startCycleId {
			for cycleId := startCycleId; cycleId <= finishCycleId; cycleId += 65536 {
				log.Println(fmt.Sprintf("Cycle id %d...", cycleId))
				for fromTs := startCycleId; fromTs <= startCycleId+65535; fromTs += 600 {
					toTs := fromTs + 60
					log.Println(fmt.Sprintf("Cycle id %d: %d - %d..", cycleId, fromTs, toTs))
					if err := s.ProcessCycles(stop, float64(effThreshold), &cycleId, fromTs, toTs, true); err != nil {
						log.Fatalf(err.Error())
						return
					}
				}
			}
		}
	} else {
		for {
			if err := s.ProcessCycles(stop, float64(effThreshold), nil, int(time.Now().Add(-60).Unix()), int(time.Now().Unix()), false); err != nil {
				log.Fatalf("Scrapper failed: %v", err)
				return
			}
		}
	}

	log.Println("Scrapper finished successfully.")
}

func runNotifier(wg *sync.WaitGroup, stop <-chan struct{}) {
	defer wg.Done()
	log.Println("Starting Notifier...")
	n, err := notifier.NewNotifier(clickhouseService, cacheService)

	if err != nil {
		log.Fatalf("Failed to initialize Notifier: %v", err)
	}
	n.ListenAndNotify(stop)
	log.Println("Notifier finished successfully.")
}

func runBackend(wg *sync.WaitGroup, stop <-chan struct{}) {
	defer wg.Done()

	h := handlers.NewHandlers(clickhouseService, cacheService)
	server := &http.Server{
		Addr:    ":3000",
		Handler: nil,
	}
	http.Handle("/", http.FileServer(http.Dir("./static")))
	http.HandleFunc("/api/chart", h.ChartHandler)
	http.HandleFunc("/api/health", h.HealthHandler)
	http.HandleFunc("/api/validator-statuses", h.ValidatorStatusesHandler)

	serverErrChan := make(chan error, 1)
	go func() {
		log.Println("Backend started on :3000")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrChan <- err
		}
	}()

	select {
	case <-stop:
		log.Println("Shutting down the backend server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Fatalf("Backend server shutdown failed: %v", err)
		} else {
			log.Println("Backend server gracefully stopped")
		}

	case err := <-serverErrChan:
		log.Fatalf("Backend server failed: %v", err)
	}
}
