package services

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"validators-health/internal/clients/clickhouse"
	. "validators-health/internal/models"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type ClickhouseService struct {
	DB driver.Conn
}

func NewClickhouseService() (*ClickhouseService, error) {
	client, err := clickhouse.GetClickHouseClient()
	if err != nil {
		return nil, err
	}
	return &ClickhouseService{DB: client.DB}, nil
}

func (s *ClickhouseService) GetValidatorsStatuses(from, to time.Time, cycleID uint32, cacheService *CacheService) (map[string]map[uint32]float64, error) {
	fromRounded, toRounded := roundTimeRange(from, to)

	adnlList, err := s.getCachedADNLList(fromRounded, toRounded, cacheService)
	if err != nil {
		return nil, err
	}

	statuses, err := s.getCachedStatuses(adnlList, fromRounded, toRounded, cycleID, cacheService)
	if err != nil {
		log.Fatalf(err.Error())
		return nil, err
	}

	return statuses, nil
}

func (s *ClickhouseService) getCachedADNLList(from, to time.Time, cacheService *CacheService) ([]string, error) {
	cacheKey := fmt.Sprintf("ADNLList:%d:%d", from.Unix(), to.Unix())
	var adnlList []string

	found, err := cacheService.GetCachedData(cacheKey, &adnlList)
	if err != nil {
		return nil, err
	}
	if found {
		return adnlList, nil
	}

	query := `
		SELECT DISTINCT adnl_addr
		FROM validator_efficiency
		WHERE date >= toDate(?) AND date <= toDate(?)
	`
	ctx := context.Background()
	rows, err := s.DB.Query(ctx, query, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var adnl string
		if err := rows.Scan(&adnl); err != nil {
			return nil, err
		}
		adnlList = append(adnlList, adnl)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := cacheService.CacheData(cacheKey, adnlList, time.Hour); err != nil {
		log.Printf("Error caching ADNL list: %v", err)
	}

	return adnlList, nil
}

func (s *ClickhouseService) getCachedStatuses(adnlList []string, from, to time.Time, cycleID uint32, cacheService *CacheService) (map[string]map[uint32]float64, error) {
	statuses := make(map[string]map[uint32]float64)
	var missingADNLs []string

	for _, adnl := range adnlList {
		cacheKey := fmt.Sprintf("ValidatorStatus:%s:%d:%d:%d", adnl, from.Unix(), to.Unix(), cycleID)
		var cachedData map[uint32]float64
		found, err := cacheService.GetCachedData(cacheKey, &cachedData)
		if err != nil {
			return nil, err
		}
		if found {
			statuses[adnl] = cachedData
		} else {
			missingADNLs = append(missingADNLs, adnl)
		}
	}

	if len(missingADNLs) > 0 {
		missingStatuses, err := s.fetchStatusesFromDB(missingADNLs, from, to, cycleID)
		if err != nil {
			return nil, err
		}

		for adnl, data := range missingStatuses {
			cacheKey := fmt.Sprintf("ValidatorStatus:%s:%d:%d:%d", adnl, from.Unix(), to.Unix(), cycleID)
			if err := cacheService.CacheChunkData(cacheKey, data, time.Hour); err != nil {
				log.Printf("Error caching statuses for %s: %v", adnl, err)
			}
			statuses[adnl] = data
		}
	}

	return statuses, nil
}

func (s *ClickhouseService) fetchStatusesFromDB(adnls []string, from, to time.Time, cycleID uint32) (map[string]map[uint32]float64, error) {
	totalSeconds := uint32(to.Sub(from).Seconds())
	if totalSeconds <= 0 {
		return nil, fmt.Errorf("invalid date interval")
	}
	intervalSeconds := totalSeconds / 60

	placeholders := make([]string, len(adnls))
	params := []interface{}{intervalSeconds, from, from, to, from, to}
	for i, adnl := range adnls {
		placeholders[i] = "?"
		params = append(params, adnl)
	}

	cycleQuery := ""
	if cycleID != 0 {
		cycleQuery = "AND cycle_id = ?"
	}

	query := fmt.Sprintf(`
		SELECT
			toUnixTimestamp(toStartOfInterval(toDateTime(timestamp), INTERVAL ? SECOND, ?)) AS interval_start,
			AVG(efficiency) AS avg_efficiency,
			adnl_addr,
			cycle_id
		FROM validator_efficiency
		WHERE
		   	date >= toDate(?) AND date <= toDate(?) 
		   and
			timestamp >= ? AND timestamp <= ?
			AND validator_efficiency.adnl_addr IN (%s)
-- 			%s
		GROUP BY adnl_addr, interval_start, cycle_id, efficiency
		ORDER BY adnl_addr, interval_start WITH FILL 
		FROM toUnixTimestamp(?) TO toUnixTimestamp(?) STEP ?
	`, strings.Join(placeholders, ","), cycleQuery)

	if cycleID != 0 {
		params = append(params, cycleID)
	}
	params = append(params, from, to, intervalSeconds)

	ctx := context.Background()
	rows, err := s.DB.Query(ctx, query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	statuses := make(map[string]map[uint32]float64)
	for rows.Next() {
		var intervalStart uint32
		var avgEfficiency float64
		var validatorAdnl string
		var cycleIDRow uint32

		if err := rows.Scan(&intervalStart, &avgEfficiency, &validatorAdnl, &cycleIDRow); err != nil {
			return nil, err
		}

		if statuses[validatorAdnl] == nil {
			statuses[validatorAdnl] = make(map[uint32]float64)
		}
		statuses[validatorAdnl][intervalStart] = avgEfficiency
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return statuses, nil
}

func (s *ClickhouseService) GetValidatorsMeta(from, to time.Time, cycleID uint32, cacheService *CacheService) (*Meta, error) {
	fromRounded, toRounded := roundTimeRange(from, to)
	cacheKey := fmt.Sprintf("GetValidatorsMeta:%d:%d", fromRounded.Unix(), toRounded.Unix())

	var meta Meta
	found, err := cacheService.GetCachedData(cacheKey, &meta)
	if err != nil {
		return nil, err
	}
	if found {
		return &meta, nil
	}

	cycleQuery := ""
	if cycleID != 0 {
		cycleQuery = "AND cycle_id = ?"
	}

	params := []interface{}{fromRounded, toRounded, fromRounded, toRounded}
	query := fmt.Sprintf(`
		SELECT
			adnl_addr,
			AVG(stake/1000000000) AS avg_stake,
			AVG(weight) AS avg_weight,
			any("index") AS index,
			v.wallet_address,
			AVG(efficiency) AS avg_efficiency,
			cycle_id
		FROM validator_efficiency
		LEFT JOIN validators v ON validators.adnl_addr = adnl_addr
		WHERE
		    date >= toDate(?) AND date <= toDate(?) 
		  and
 			timestamp >= ? AND timestamp <= ? 
			%s
		GROUP BY adnl_addr, wallet_address, cycle_id
		ORDER BY avg_stake DESC
	`, cycleQuery)

	if cycleID != 0 {
		params = append(params, cycleID)
	}

	ctx := context.Background()
	rows, err := s.DB.Query(ctx, query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	meta = Meta{}
	for rows.Next() {
		var adnl string
		var avgStake float64
		var avgWeight float64
		var index uint16
		var walletAddress string
		var avgEfficiency float64
		var cycleIDRow uint32

		if err := rows.Scan(&adnl, &avgStake, &avgWeight, &index, &walletAddress, &avgEfficiency, &cycleIDRow); err != nil {
			return nil, err
		}

		meta[adnl] = struct {
			Weight        string  `json:"weight"`
			Index         uint16  `json:"index"`
			Stake         string  `json:"stake"`
			WalletAddress string  `json:"wallet_address"`
			AvgEfficiency float64 `json:"avg_efficiency"`
			CycleID       uint32  `json:"cycle_id"`
		}{
			Stake:         strconv.Itoa(int(avgStake)),
			Weight:        strconv.Itoa(int(avgWeight)),
			Index:         index,
			WalletAddress: walletAddress,
			AvgEfficiency: avgEfficiency,
			CycleID:       cycleIDRow,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := cacheService.CacheData(cacheKey, meta, time.Hour); err != nil {
		log.Printf("Error caching validator meta: %v", err)
	}

	return &meta, nil
}

func (s *ClickhouseService) GetEfficiencyChartDataCached(adnl string, from, to time.Time, cacheService *CacheService) ([]ValidatorEfficiency, error) {
	fromRounded, toRounded := roundTimeRange(from, to)
	cacheKey := fmt.Sprintf("GetEfficiencyChartDataCached:%s:%d:%d", adnl, fromRounded.Unix(), toRounded.Unix())

	var results []ValidatorEfficiency
	found, err := cacheService.GetCachedData(cacheKey, &results)
	if err != nil {
		return nil, err
	}
	if found {
		return results, nil
	}

	results, err = s.fetchEfficiencyChartDataFromDB(adnl, fromRounded, toRounded)
	if err != nil {
		return nil, err
	}

	if err := cacheService.CacheData(cacheKey, results, time.Hour); err != nil {
		log.Printf("Error caching efficiency chart data for %s: %v", adnl, err)
	}

	return results, nil
}

func (s *ClickhouseService) fetchEfficiencyChartDataFromDB(adnl string, from, to time.Time) ([]ValidatorEfficiency, error) {
	fromRounded, toRounded := roundTimeRange(from, to)

	totalSeconds := uint32(toRounded.Sub(fromRounded).Seconds())
	if totalSeconds <= 0 {
		return nil, fmt.Errorf("invalid date interval")
	}
	intervalSeconds := totalSeconds / 60

	query := `
		SELECT toUnixTimestamp(toStartOfInterval(toDateTime(timestamp), INTERVAL ? SECOND, ?)) AS interval_start,
			   AVG(efficiency) as avg_efficiency,
			   cycle_id
		FROM validator_efficiency
		WHERE adnl_addr = ?
		  AND timestamp BETWEEN ? AND ?
		GROUP BY interval_start, cycle_id
		ORDER BY interval_start
	`
	ctx := context.Background()
	rows, err := s.DB.Query(ctx, query, intervalSeconds, fromRounded, adnl, fromRounded, toRounded)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var efficiencies []ValidatorEfficiency
	for rows.Next() {
		var eff ValidatorEfficiency
		if err := rows.Scan(&eff.IntervalStart, &eff.Efficiency, &eff.CycleID); err != nil {
			return nil, err
		}
		eff.ValidatorADNL = adnl
		efficiencies = append(efficiencies, eff)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return efficiencies, nil
}

func (s *ClickhouseService) GetStatusHistory(adnl string, cacheService *CacheService) ([]ValidatorStatusHistory, error) {
	cacheKey := fmt.Sprintf("status_history:%s", adnl)
	var history []ValidatorStatusHistory

	found, err := cacheService.GetCachedData(cacheKey, &history)
	if err != nil {
		return nil, err
	}
	if found {
		return history, nil
	}

	query := `
		SELECT timestamp, status
		FROM validator_status_history
		WHERE adnl_addr = ?
		ORDER BY timestamp DESC
		LIMIT 1000
	`
	ctx := context.Background()
	rows, err := s.DB.Query(ctx, query, adnl)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var record ValidatorStatusHistory
		if err := rows.Scan(&record.Timestamp, &record.Status); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		history = append(history, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := cacheService.CacheData(cacheKey, history, time.Hour); err != nil {
		log.Printf("Error caching status history for %s: %v", adnl, err)
	}

	return history, nil
}

func (s *ClickhouseService) InsertScoreboard(scoreboard []CycleScoreboardRow, timeStamp int64) error {
	ctx := context.Background()
	batch, err := s.DB.PrepareBatch(ctx, "INSERT INTO validator_efficiency (timestamp, validator_adnl, adnl_addr, cycle_id, efficiency, stake, weight, index, pub_key_hash, utime_since, utime_until)")
	if err != nil {
		return err
	}

	for _, row := range scoreboard {
		if err := batch.Append(timeStamp, row.ValidatorADNL, row.ADNLAddr, row.CycleID, row.Efficiency, row.Stake, row.Weight, row.Index, row.PubKeyHash, row.UtimeSince, row.UtimeUntil); err != nil {
			return err
		}
	}

	return batch.Send()
}

func (s *ClickhouseService) InsertStatusChange(adnlAddr string, validatorAdnl string, status ValidatorStatus, timestamp time.Time) error {
	query := "INSERT INTO validator_status_history (adnl_addr, validator_adnl, timestamp, status) VALUES (?, ?, ?, ?)"
	ctx := context.Background()
	err := s.DB.Exec(ctx, query, adnlAddr, validatorAdnl, timestamp.Unix(), string(status))
	if err != nil {
		return fmt.Errorf("error inserting status change into ClickHouse: %w", err)
	}
	return nil
}

func (s *ClickhouseService) InsertCycles(cycles []Cycle) error {
	ctx := context.Background()
	batch, err := s.DB.PrepareBatch(ctx, "INSERT INTO cycles (cycle_id)")
	if err != nil {
		return fmt.Errorf("failed to prepare batch for cycles: %w", err)
	}

	for _, cycle := range cycles {
		if err := batch.Append(cycle.CycleID); err != nil {
			return fmt.Errorf("failed to append cycle: %w", err)
		}
	}

	return batch.Send()
}

func (s *ClickhouseService) InsertCyclesInfo(cycles []Cycle) error {
	ctx := context.Background()
	batch, err := s.DB.PrepareBatch(ctx, "INSERT INTO cycles_info (cycle_id, utime_since, utime_until, total_weight)")
	if err != nil {
		return fmt.Errorf("failed to prepare batch for cycles_info: %w", err)
	}

	for _, cycle := range cycles {
		if err := batch.Append(
			cycle.CycleID,
			time.Unix(cycle.CycleInfo.UtimeSince, 0),
			time.Unix(cycle.CycleInfo.UtimeUntil, 0),
			cycle.CycleInfo.TotalWeight,
		); err != nil {
			return fmt.Errorf("failed to append cycle info: %w", err)
		}
	}

	return batch.Send()
}

func (s *ClickhouseService) InsertValidators(cycles []Cycle) error {
	ctx := context.Background()
	batch, err := s.DB.PrepareBatch(ctx, "INSERT INTO validators (cycle_id, adnl_addr, pubkey, weight, index, stake, max_factor, wallet_address)")
	if err != nil {
		return fmt.Errorf("failed to prepare batch for validators: %w", err)
	}

	for _, cycle := range cycles {
		for _, validator := range cycle.CycleInfo.Validators {
			if err := batch.Append(
				cycle.CycleID,
				validator.ADNLAddr,
				validator.PubKey,
				validator.Weight,
				validator.Index,
				validator.Stake,
				validator.MaxFactor,
				validator.WalletAddress,
			); err != nil {
				return fmt.Errorf("failed to append validator: %w", err)
			}
		}
	}

	return batch.Send()
}

func roundTimeRange(from, to time.Time) (time.Time, time.Time) {
	fromRounded := from.Round(time.Minute)
	toRounded := to.Round(time.Minute)

	if time.Now().Before(toRounded) {
		diff := toRounded.Sub(fromRounded)
		toRounded = time.Now().Round(time.Minute).Add(-time.Minute)
		fromRounded = toRounded.Add(-diff)
	}

	return fromRounded, toRounded
}
