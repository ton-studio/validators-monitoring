package clickhouse

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type Client struct {
	DB driver.Conn
}

var (
	clickhouseInstance *Client
	clickhouseOnce     sync.Once
	clickhouseInitErr  error
)

func GetClickHouseClient() (*Client, error) {
	clickhouseOnce.Do(func() {
		var conn driver.Conn
		var err error
		retries := 5
		initialBackoff := time.Second * 2

		for i := 0; i < retries; i++ {
			conn, err = clickhouse.Open(&clickhouse.Options{
				Addr: []string{os.Getenv("CLICKHOUSE_HOST")},
				Auth: clickhouse.Auth{
					Database: os.Getenv("CLICKHOUSE_DB"),
					Username: os.Getenv("CLICKHOUSE_USER"),
					Password: os.Getenv("CLICKHOUSE_PASSWORD"),
				},
				Debug: true,
			})
			if err != nil {
				log.Printf("Couldn't open connection to ClickHouse (try %d/%d): %v", i+1, retries, err)
				time.Sleep(initialBackoff)
				initialBackoff *= 2
				continue
			}

			ctx := context.Background()
			if err = conn.Ping(ctx); err != nil {
				log.Printf("Couldn't connect to ClickHouse (try %d/%d): %v", i+1, retries, err)
				err := conn.Close()
				if err != nil {
					log.Printf("Couldn't close connection to ClickHouse: %v", err)
					return
				}
				time.Sleep(initialBackoff)
				initialBackoff *= 2
				continue
			}

			clickhouseInstance = &Client{DB: conn}
			log.Println("Connected to ClickHouse successfully.")
			return
		}

		clickhouseInitErr = err
	})

	return clickhouseInstance, clickhouseInitErr
}

func (c *Client) Close() error {
	return c.DB.Close()
}
