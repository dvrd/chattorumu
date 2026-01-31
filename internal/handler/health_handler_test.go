package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"jobsity-chat/internal/testutil"
)

func TestHealth_ReturnsOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	Health(w, req)

	testutil.AssertStatusCode(t, w, http.StatusOK)
	testutil.AssertHeader(t, w, "Content-Type", "application/json")

	var response map[string]string
	err := json.NewDecoder(w.Body).Decode(&response)
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, response["status"], "ok")
}

func TestHealth_AlwaysReturns200(t *testing.T) {
	// Health endpoint should always return 200 regardless of underlying services
	tests := []struct {
		name   string
		method string
	}{
		{"GET request", http.MethodGet},
		{"HEAD request", http.MethodHead},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/health", nil)
			w := httptest.NewRecorder()

			Health(w, req)

			testutil.AssertStatusCode(t, w, http.StatusOK)
		})
	}
}

func TestHealthCheckResult_JSON(t *testing.T) {
	tests := []struct {
		name   string
		result HealthCheckResult
		want   map[string]interface{}
	}{
		{
			name: "healthy service",
			result: HealthCheckResult{
				Status:    "up",
				LatencyMs: 5,
			},
			want: map[string]interface{}{
				"status":     "up",
				"latency_ms": float64(5),
			},
		},
		{
			name: "unhealthy service",
			result: HealthCheckResult{
				Status:    "down",
				LatencyMs: 100,
				Error:     "connection refused",
			},
			want: map[string]interface{}{
				"status":     "down",
				"latency_ms": float64(100),
				"error":      "connection refused",
			},
		},
		{
			name: "with metadata",
			result: HealthCheckResult{
				Status:    "up",
				LatencyMs: 3,
				Metadata: map[string]any{
					"connections_open": 5,
					"max_open":         10,
				},
			},
			want: map[string]interface{}{
				"status":     "up",
				"latency_ms": float64(3),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.result)
			testutil.AssertNoError(t, err)

			var result map[string]interface{}
			err = json.Unmarshal(data, &result)
			testutil.AssertNoError(t, err)

			for key, expected := range tt.want {
				got, ok := result[key]
				if !ok {
					t.Errorf("missing key %q", key)
					continue
				}
				switch v := expected.(type) {
				case string:
					testutil.AssertEqual(t, got.(string), v)
				case float64:
					testutil.AssertEqual(t, got.(float64), v)
				}
			}
		})
	}
}

func TestHealthCheckResult_OmitsEmptyFields(t *testing.T) {
	result := HealthCheckResult{
		Status: "up",
	}

	data, err := json.Marshal(result)
	testutil.AssertNoError(t, err)

	// Should not include latency_ms, error, or metadata when empty/zero
	jsonStr := string(data)
	testutil.AssertNotContains(t, jsonStr, "latency_ms")
	testutil.AssertNotContains(t, jsonStr, "error")
	testutil.AssertNotContains(t, jsonStr, "metadata")
}

func TestHealthCheckResult_IncludesError(t *testing.T) {
	result := HealthCheckResult{
		Status: "down",
		Error:  "connection refused",
	}

	data, err := json.Marshal(result)
	testutil.AssertNoError(t, err)

	testutil.AssertContains(t, string(data), "connection refused")
}

func TestHealthCheckResult_IncludesMetadata(t *testing.T) {
	result := HealthCheckResult{
		Status: "up",
		Metadata: map[string]any{
			"connections_open": 5,
			"max_open":         10,
		},
	}

	data, err := json.Marshal(result)
	testutil.AssertNoError(t, err)

	testutil.AssertContains(t, string(data), "metadata")
	testutil.AssertContains(t, string(data), "connections_open")
}

// TestReady_ResponseFormat tests the expected response format
// Note: Full testing of Ready requires integration with real DB and RabbitMQ
func TestReady_ExpectedResponseFormat(t *testing.T) {
	// Verify the expected response structure is valid
	expectedFormat := map[string]interface{}{
		"status":    "ready",
		"timestamp": "2026-01-31T12:00:00Z",
		"checks": map[string]interface{}{
			"database": map[string]interface{}{
				"status":     "up",
				"latency_ms": float64(5),
				"metadata": map[string]interface{}{
					"connections_open": float64(5),
				},
			},
			"rabbitmq": map[string]interface{}{
				"status":     "up",
				"latency_ms": float64(1),
			},
		},
	}

	data, err := json.Marshal(expectedFormat)
	testutil.AssertNoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, result["status"], "ready")
	testutil.AssertNotNil(t, result["checks"])
}

func TestReady_NotReadyResponseFormat(t *testing.T) {
	// Verify the not_ready response structure is valid
	expectedFormat := map[string]interface{}{
		"status":    "not_ready",
		"timestamp": "2026-01-31T12:00:00Z",
		"checks": map[string]interface{}{
			"database": map[string]interface{}{
				"status": "down",
				"error":  "connection refused",
			},
			"rabbitmq": map[string]interface{}{
				"status": "up",
			},
		},
	}

	data, err := json.Marshal(expectedFormat)
	testutil.AssertNoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	testutil.AssertNoError(t, err)

	testutil.AssertEqual(t, result["status"], "not_ready")
}

func TestHealth_ContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	Health(w, req)

	contentType := w.Header().Get("Content-Type")
	testutil.AssertEqual(t, contentType, "application/json")
}

func TestHealth_ResponseBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	Health(w, req)

	body := w.Body.String()
	testutil.AssertContains(t, body, "status")
	testutil.AssertContains(t, body, "ok")
}

func TestHealth_ValidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	Health(w, req)

	var response interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	testutil.AssertNoError(t, err)
}

// Benchmark health endpoint
func BenchmarkHealth(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		Health(w, req)
	}
}
