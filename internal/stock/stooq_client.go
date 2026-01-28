package stock

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

var (
	ErrStockNotFound   = errors.New("stock not found")
	ErrInvalidResponse = errors.New("invalid response from Stooq API")
)

// Quote represents a stock quote
type Quote struct {
	Symbol string
	Price  float64
	Date   string
	Time   string
}

// StooqClient handles requests to the Stooq API
type StooqClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewStooqClient creates a new Stooq API client
func NewStooqClient(baseURL string) *StooqClient {
	return &StooqClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetQuote fetches a stock quote from Stooq API
func (c *StooqClient) GetQuote(ctx context.Context, stockCode string) (*Quote, error) {
	url := fmt.Sprintf("%s/q/l/?s=%s&f=sd2t2ohlcv&h&e=csv", c.baseURL, stockCode)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Retry logic with exponential backoff
	var resp *http.Response
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		resp, lastErr = c.httpClient.Do(req)
		if lastErr == nil && resp.StatusCode == http.StatusOK {
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		if attempt < 3 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("failed to fetch quote after 3 attempts: %w", lastErr)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	// Parse CSV response
	return c.parseCSV(resp.Body)
}

// parseCSV parses the CSV response from Stooq API
func (c *StooqClient) parseCSV(body io.Reader) (*Quote, error) {
	reader := csv.NewReader(body)

	// Read all records
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV: %w", err)
	}

	// Expected format:
	// Header: Symbol,Date,Time,Open,High,Low,Close,Volume
	// Data:   AAPL.US,2026-01-28,22:00:00,150.0,152.0,149.0,151.5,1000000

	if len(records) < 2 {
		return nil, ErrInvalidResponse
	}

	// Get data row (second row)
	data := records[1]
	if len(data) < 7 {
		return nil, ErrInvalidResponse
	}

	symbol := data[0]
	date := data[1]
	time := data[2]
	closePrice := data[6]

	// Check for N/D (not available)
	if closePrice == "N/D" || symbol == "N/D" {
		return nil, ErrStockNotFound
	}

	// Parse closing price
	price, err := strconv.ParseFloat(closePrice, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse price: %w", err)
	}

	return &Quote{
		Symbol: symbol,
		Price:  price,
		Date:   date,
		Time:   time,
	}, nil
}
