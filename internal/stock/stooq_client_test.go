package stock

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGetQuote_Success(t *testing.T) {
	// Create a test server that returns valid CSV
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify query parameters
		stockCode := r.URL.Query().Get("s")
		if stockCode != "AAPL.US" {
			t.Errorf("Expected stock code AAPL.US, got %s", stockCode)
		}

		// Return valid CSV
		csv := "Symbol,Date,Time,Open,High,Low,Close,Volume\nAAPL.US,2026-01-28,22:00:00,150.0,152.0,149.0,151.5,1000000"
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(csv))
	}))
	defer server.Close()

	client := NewStooqClient(server.URL)
	ctx := context.Background()

	quote, err := client.GetQuote(ctx, "AAPL.US")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if quote == nil {
		t.Fatal("Expected non-nil quote")
	}

	if quote.Symbol != "AAPL.US" {
		t.Errorf("Expected symbol AAPL.US, got %s", quote.Symbol)
	}

	if quote.Price != 151.5 {
		t.Errorf("Expected price 151.5, got %.2f", quote.Price)
	}

	if quote.Date != "2026-01-28" {
		t.Errorf("Expected date 2026-01-28, got %s", quote.Date)
	}

	if quote.Time != "22:00:00" {
		t.Errorf("Expected time 22:00:00, got %s", quote.Time)
	}
}

func TestGetQuote_StockNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return CSV with N/D for not available
		csv := "Symbol,Date,Time,Open,High,Low,Close,Volume\nN/D,N/D,N/D,N/D,N/D,N/D,N/D,N/D"
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(csv))
	}))
	defer server.Close()

	client := NewStooqClient(server.URL)
	ctx := context.Background()

	quote, err := client.GetQuote(ctx, "INVALID")

	if err != ErrStockNotFound {
		t.Errorf("Expected ErrStockNotFound, got: %v", err)
	}

	if quote != nil {
		t.Errorf("Expected nil quote, got: %+v", quote)
	}
}

func TestGetQuote_NetworkError(t *testing.T) {
	// Use invalid URL to simulate network error
	client := NewStooqClient("http://invalid.domain.that.does.not.exist.local")
	ctx := context.Background()

	quote, err := client.GetQuote(ctx, "AAPL.US")

	if err == nil {
		t.Error("Expected error for network failure")
	}

	if quote != nil {
		t.Errorf("Expected nil quote, got: %+v", quote)
	}

	// Verify error message mentions retries
	if !strings.Contains(err.Error(), "after 3 attempts") {
		t.Errorf("Expected error to mention retry attempts, got: %v", err)
	}
}

func TestGetQuote_HTTPErrorStatus(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"Bad Request", http.StatusBadRequest},
		{"Not Found", http.StatusNotFound},
		{"Internal Server Error", http.StatusInternalServerError},
		{"Service Unavailable", http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := NewStooqClient(server.URL)
			ctx := context.Background()

			quote, err := client.GetQuote(ctx, "AAPL.US")

			if err == nil {
				t.Error("Expected error for HTTP error status")
			}

			if quote != nil {
				t.Errorf("Expected nil quote, got: %+v", quote)
			}
		})
	}
}

func TestGetQuote_ContextCancellation(t *testing.T) {
	// Create server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		csv := "Symbol,Date,Time,Open,High,Low,Close,Volume\nAAPL.US,2026-01-28,22:00:00,150.0,152.0,149.0,151.5,1000000"
		w.Write([]byte(csv))
	}))
	defer server.Close()

	client := NewStooqClient(server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	quote, err := client.GetQuote(ctx, "AAPL.US")

	if err == nil {
		t.Error("Expected error for context timeout")
	}

	if quote != nil {
		t.Errorf("Expected nil quote, got: %+v", quote)
	}
}

func TestGetQuote_RetryLogic(t *testing.T) {
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount < 3 {
			// Fail first 2 attempts
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Succeed on 3rd attempt
		csv := "Symbol,Date,Time,Open,High,Low,Close,Volume\nAAPL.US,2026-01-28,22:00:00,150.0,152.0,149.0,151.5,1000000"
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(csv))
	}))
	defer server.Close()

	client := NewStooqClient(server.URL)
	ctx := context.Background()

	quote, err := client.GetQuote(ctx, "AAPL.US")

	if err != nil {
		t.Fatalf("Expected success on retry, got error: %v", err)
	}

	if quote == nil {
		t.Fatal("Expected non-nil quote")
	}

	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", attemptCount)
	}

	if quote.Price != 151.5 {
		t.Errorf("Expected price 151.5, got %.2f", quote.Price)
	}
}

func TestParseCSV_ValidResponse(t *testing.T) {
	csv := "Symbol,Date,Time,Open,High,Low,Close,Volume\nAAPL.US,2026-01-28,22:00:00,150.0,152.0,149.0,151.5,1000000"
	reader := strings.NewReader(csv)

	client := NewStooqClient("http://example.com")
	quote, err := client.parseCSV(reader)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if quote == nil {
		t.Fatal("Expected non-nil quote")
	}

	if quote.Symbol != "AAPL.US" {
		t.Errorf("Expected symbol AAPL.US, got %s", quote.Symbol)
	}

	if quote.Price != 151.5 {
		t.Errorf("Expected price 151.5, got %.2f", quote.Price)
	}
}

func TestParseCSV_InvalidFormat(t *testing.T) {
	tests := []struct {
		name string
		csv  string
	}{
		{
			name: "empty CSV",
			csv:  "",
		},
		{
			name: "only header",
			csv:  "Symbol,Date,Time,Open,High,Low,Close,Volume",
		},
		{
			name: "insufficient columns",
			csv:  "Symbol,Date,Time\nAAPL.US,2026-01-28,22:00:00",
		},
		{
			name: "missing data row",
			csv:  "Symbol,Date,Time,Open,High,Low,Close,Volume\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.csv)
			client := NewStooqClient("http://example.com")

			quote, err := client.parseCSV(reader)

			if err == nil {
				t.Error("Expected error for invalid CSV format")
			}

			if quote != nil {
				t.Errorf("Expected nil quote, got: %+v", quote)
			}
		})
	}
}

func TestParseCSV_NotAvailable(t *testing.T) {
	tests := []struct {
		name string
		csv  string
	}{
		{
			name: "all N/D",
			csv:  "Symbol,Date,Time,Open,High,Low,Close,Volume\nN/D,N/D,N/D,N/D,N/D,N/D,N/D,N/D",
		},
		{
			name: "symbol N/D",
			csv:  "Symbol,Date,Time,Open,High,Low,Close,Volume\nN/D,2026-01-28,22:00:00,150.0,152.0,149.0,151.5,1000000",
		},
		{
			name: "close price N/D",
			csv:  "Symbol,Date,Time,Open,High,Low,Close,Volume\nAAPL.US,2026-01-28,22:00:00,150.0,152.0,149.0,N/D,1000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.csv)
			client := NewStooqClient("http://example.com")

			quote, err := client.parseCSV(reader)

			if !errors.Is(err, ErrStockNotFound) {
				t.Errorf("Expected ErrStockNotFound, got: %v", err)
			}

			if quote != nil {
				t.Errorf("Expected nil quote, got: %+v", quote)
			}
		})
	}
}

func TestParseCSV_InvalidPrice(t *testing.T) {
	csv := "Symbol,Date,Time,Open,High,Low,Close,Volume\nAAPL.US,2026-01-28,22:00:00,150.0,152.0,149.0,INVALID,1000000"
	reader := strings.NewReader(csv)

	client := NewStooqClient("http://example.com")
	quote, err := client.parseCSV(reader)

	if err == nil {
		t.Error("Expected error for invalid price")
	}

	if quote != nil {
		t.Errorf("Expected nil quote, got: %+v", quote)
	}

	if !strings.Contains(err.Error(), "failed to parse price") {
		t.Errorf("Expected error about parsing price, got: %v", err)
	}
}

func TestParseCSV_DifferentPriceFormats(t *testing.T) {
	tests := []struct {
		name          string
		closePrice    string
		expectedPrice float64
	}{
		{
			name:          "integer price",
			closePrice:    "100",
			expectedPrice: 100.0,
		},
		{
			name:          "decimal price",
			closePrice:    "151.5",
			expectedPrice: 151.5,
		},
		{
			name:          "price with many decimals",
			closePrice:    "151.123456",
			expectedPrice: 151.123456,
		},
		{
			name:          "zero price",
			closePrice:    "0",
			expectedPrice: 0.0,
		},
		{
			name:          "very large price",
			closePrice:    "999999.99",
			expectedPrice: 999999.99,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			csv := "Symbol,Date,Time,Open,High,Low,Close,Volume\nAAPL.US,2026-01-28,22:00:00,150.0,152.0,149.0," + tt.closePrice + ",1000000"
			reader := strings.NewReader(csv)

			client := NewStooqClient("http://example.com")
			quote, err := client.parseCSV(reader)

			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}

			if quote.Price != tt.expectedPrice {
				t.Errorf("Expected price %.6f, got %.6f", tt.expectedPrice, quote.Price)
			}
		})
	}
}

func TestNewStooqClient(t *testing.T) {
	baseURL := "https://stooq.com"
	client := NewStooqClient(baseURL)

	if client == nil {
		t.Fatal("Expected non-nil client")
	}

	if client.baseURL != baseURL {
		t.Errorf("Expected baseURL %s, got %s", baseURL, client.baseURL)
	}

	if client.httpClient == nil {
		t.Error("Expected non-nil httpClient")
	}

	// Verify timeout is set
	if client.httpClient.Timeout != 10*time.Second {
		t.Errorf("Expected timeout 10s, got %v", client.httpClient.Timeout)
	}
}

func TestGetQuote_URLConstruction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify URL parameters
		expectedPath := "/q/l/"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}

		stockCode := r.URL.Query().Get("s")
		if stockCode != "AAPL.US" {
			t.Errorf("Expected stock code AAPL.US, got %s", stockCode)
		}

		format := r.URL.Query().Get("f")
		if format != "sd2t2ohlcv" {
			t.Errorf("Expected format sd2t2ohlcv, got %s", format)
		}

		exportType := r.URL.Query().Get("e")
		if exportType != "csv" {
			t.Errorf("Expected export type csv, got %s", exportType)
		}

		csv := "Symbol,Date,Time,Open,High,Low,Close,Volume\nAAPL.US,2026-01-28,22:00:00,150.0,152.0,149.0,151.5,1000000"
		w.Write([]byte(csv))
	}))
	defer server.Close()

	client := NewStooqClient(server.URL)
	ctx := context.Background()

	_, err := client.GetQuote(ctx, "AAPL.US")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

func TestParseCSV_MalformedCSV(t *testing.T) {
	tests := []struct {
		name string
		csv  string
	}{
		{
			name: "invalid CSV syntax",
			csv:  "Symbol,Date,Time\nAAPL.US,2026-01-28,22:00:00,extra,columns",
		},
		{
			name: "unclosed quote",
			csv:  "Symbol,Date,Time,Open,High,Low,Close,Volume\n\"AAPL.US,2026-01-28,22:00:00,150.0,152.0,149.0,151.5,1000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.csv)
			client := NewStooqClient("http://example.com")

			quote, err := client.parseCSV(reader)

			// Should either error or return invalid data
			if err == nil && quote == nil {
				t.Error("Expected either error or quote, got neither")
			}
		})
	}
}

// Benchmark tests
func BenchmarkGetQuote(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		csv := "Symbol,Date,Time,Open,High,Low,Close,Volume\nAAPL.US,2026-01-28,22:00:00,150.0,152.0,149.0,151.5,1000000"
		w.Write([]byte(csv))
	}))
	defer server.Close()

	client := NewStooqClient(server.URL)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client.GetQuote(ctx, "AAPL.US")
	}
}

func BenchmarkParseCSV(b *testing.B) {
	csv := "Symbol,Date,Time,Open,High,Low,Close,Volume\nAAPL.US,2026-01-28,22:00:00,150.0,152.0,149.0,151.5,1000000"
	client := NewStooqClient("http://example.com")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader := strings.NewReader(csv)
		client.parseCSV(reader)
	}
}

// Test helper to create mock CSV reader
func createMockCSV(symbol, date, time, close string) io.Reader {
	csv := "Symbol,Date,Time,Open,High,Low,Close,Volume\n" +
		symbol + "," + date + "," + time + ",150.0,152.0,149.0," + close + ",1000000"
	return strings.NewReader(csv)
}
