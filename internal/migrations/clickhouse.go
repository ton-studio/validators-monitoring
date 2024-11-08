package migrations

import (
	"context"
	"fmt"
	"os"

	"github.com/ClickHouse/clickhouse-go/v2"
)

func CreateTables() error {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{os.Getenv("CLICKHOUSE_HOST")},
		Auth: clickhouse.Auth{
			Database: os.Getenv("CLICKHOUSE_DB"),
			Username: os.Getenv("CLICKHOUSE_USER"),
			Password: os.Getenv("CLICKHOUSE_PASSWORD"),
		},
		Debug: true,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to ClickHouse: %w", err)
	}

	ctx := context.Background()

	queries := []string{

		`
		CREATE TABLE IF NOT EXISTS validator_efficiency
		(
			date       			Date DEFAULT toDate(timestamp),
			timestamp  			DateTime64(3),
			adnl_addr       	String,
			validator_adnl      String,
			cycle_id   			UInt32,
			stake      			Int64,
			efficiency 			Float64,
			weight 				UInt64,
			"index" 			UInt16,
			pub_key_hash 		String,
		    utime_since 		DateTime,
			utime_until 		DateTime,
			INDEX adnl_index validator_adnl TYPE bloom_filter() GRANULARITY 16
		)
		ENGINE = MergeTree
		PARTITION BY toYYYYMMDD(timestamp)
		PRIMARY KEY validator_adnl
		ORDER BY (validator_adnl, cycle_id, timestamp)
		SETTINGS index_granularity = 8192;
		`,

		`
		CREATE TABLE IF NOT EXISTS validator_status_history (
			adnl_addr 				String,
			validator_adnl       	String,
			timestamp 				DateTime,
			status 					String
		) ENGINE = MergeTree()
		ORDER BY (validator_adnl, timestamp);
		`,

		`CREATE TABLE IF NOT EXISTS cycles
		(
		cycle_id UInt32
		)
		ENGINE = ReplacingMergeTree()
		PRIMARY KEY cycle_id
		ORDER BY cycle_id;
		`,

		`
		CREATE TABLE IF NOT EXISTS cycles_info
		(
		cycle_id    UInt32,
		utime_since DateTime,
		utime_until DateTime,
		total_weight Int64
		)
		ENGINE = ReplacingMergeTree()
		PRIMARY KEY cycle_id
		ORDER BY cycle_id;
		`,

		`
		CREATE TABLE IF NOT EXISTS validators
		(
		cycle_id        UInt32,
		adnl_addr       String,
		pubkey          String,
		weight          Int64,
		"index"         UInt16,
		stake           Int64,
		max_factor      Int32,
		wallet_address  String
		)
		ENGINE = ReplacingMergeTree()
		PRIMARY KEY (cycle_id, adnl_addr)
		ORDER BY (cycle_id, adnl_addr);
		`,
	}

	for idx, query := range queries {
		if err := conn.Exec(ctx, query); err != nil {
			return fmt.Errorf("failed to execute query %d: %w", idx+1, err)
		}
	}

	return nil
}
