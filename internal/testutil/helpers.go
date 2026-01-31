package testutil

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// AssertNoError fails the test if err is not nil
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// AssertError fails the test if err is nil
func AssertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// AssertErrorIs fails the test if err does not match the expected error
func AssertErrorIs(t *testing.T, err, expected error) {
	t.Helper()
	if !errors.Is(err, expected) {
		t.Errorf("expected error %v, got: %v", expected, err)
	}
}

// AssertErrorContains fails the test if err message does not contain the substring
func AssertErrorContains(t *testing.T, err error, substring string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", substring)
	}
	if !strings.Contains(err.Error(), substring) {
		t.Errorf("expected error containing %q, got: %v", substring, err)
	}
}

// isNilable returns true if the reflect kind can be nil
func isNilable(kind reflect.Kind) bool {
	switch kind {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return true
	}
	return false
}

// AssertNil fails the test if v is not nil
func AssertNil(t *testing.T, v interface{}) {
	t.Helper()
	if v == nil {
		return
	}
	rv := reflect.ValueOf(v)
	if isNilable(rv.Kind()) && rv.IsNil() {
		return
	}
	t.Errorf("expected nil, got: %v", v)
}

// AssertNotNil fails the test if v is nil
func AssertNotNil(t *testing.T, v interface{}) {
	t.Helper()
	if v == nil {
		t.Fatal("expected non-nil value, got nil")
		return
	}
	rv := reflect.ValueOf(v)
	if isNilable(rv.Kind()) && rv.IsNil() {
		t.Fatal("expected non-nil value, got nil")
	}
}

// AssertEqual fails the test if got != want
func AssertEqual[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// AssertNotEqual fails the test if got == want
func AssertNotEqual[T comparable](t *testing.T, got, notWant T) {
	t.Helper()
	if got == notWant {
		t.Errorf("got %v, did not want %v", got, notWant)
	}
}

// AssertTrue fails the test if condition is false
func AssertTrue(t *testing.T, condition bool, msg string) {
	t.Helper()
	if !condition {
		t.Errorf("expected true: %s", msg)
	}
}

// AssertFalse fails the test if condition is true
func AssertFalse(t *testing.T, condition bool, msg string) {
	t.Helper()
	if condition {
		t.Errorf("expected false: %s", msg)
	}
}

// AssertContains fails if s does not contain substring
func AssertContains(t *testing.T, s, substring string) {
	t.Helper()
	if !strings.Contains(s, substring) {
		t.Errorf("expected %q to contain %q", s, substring)
	}
}

// AssertNotContains fails if s contains substring
func AssertNotContains(t *testing.T, s, substring string) {
	t.Helper()
	if strings.Contains(s, substring) {
		t.Errorf("expected %q to not contain %q", s, substring)
	}
}

// HTTP Test Helpers

// AssertStatusCode fails if the response status code doesn't match expected
func AssertStatusCode(t *testing.T, w *httptest.ResponseRecorder, expected int) {
	t.Helper()
	if w.Code != expected {
		t.Errorf("expected status %d, got %d. Body: %s", expected, w.Code, w.Body.String())
	}
}

// AssertJSONResponse fails if the response is not valid JSON or status doesn't match
func AssertJSONResponse(t *testing.T, w *httptest.ResponseRecorder, expectedStatus int) map[string]interface{} {
	t.Helper()
	AssertStatusCode(t, w, expectedStatus)

	var result map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode JSON response: %v. Body: %s", err, w.Body.String())
	}
	return result
}

// AssertJSONContains fails if the JSON response doesn't contain the expected key-value pair
func AssertJSONContains(t *testing.T, w *httptest.ResponseRecorder, key string, expected interface{}) {
	t.Helper()

	var result map[string]interface{}
	body := w.Body.String()
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("failed to decode JSON response: %v. Body: %s", err, body)
	}

	got, ok := result[key]
	if !ok {
		t.Errorf("JSON response missing key %q. Body: %s", key, body)
		return
	}

	if !reflect.DeepEqual(got, expected) {
		t.Errorf("JSON key %q: got %v (%T), want %v (%T)", key, got, got, expected, expected)
	}
}

// AssertJSONError fails if the response doesn't contain an error field with the expected message
func AssertJSONError(t *testing.T, w *httptest.ResponseRecorder, expectedStatus int, expectedMsg string) {
	t.Helper()
	AssertStatusCode(t, w, expectedStatus)

	body := w.Body.String()
	if !strings.Contains(body, expectedMsg) {
		t.Errorf("expected error message %q in response, got: %s", expectedMsg, body)
	}
}

// AssertHeader fails if the response header doesn't match expected value
func AssertHeader(t *testing.T, w *httptest.ResponseRecorder, key, expected string) {
	t.Helper()
	got := w.Header().Get(key)
	if got != expected {
		t.Errorf("header %q: got %q, want %q", key, got, expected)
	}
}

// AssertHeaderContains fails if the response header doesn't contain expected substring
func AssertHeaderContains(t *testing.T, w *httptest.ResponseRecorder, key, substring string) {
	t.Helper()
	got := w.Header().Get(key)
	if !strings.Contains(got, substring) {
		t.Errorf("header %q: expected to contain %q, got %q", key, substring, got)
	}
}

// AssertCookie fails if the response doesn't have a cookie with the expected name
func AssertCookie(t *testing.T, w *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	cookies := w.Result().Cookies()
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	t.Errorf("expected cookie %q not found", name)
	return nil
}

// AssertNoCookie fails if the response has a cookie with the given name
func AssertNoCookie(t *testing.T, w *httptest.ResponseRecorder, name string) {
	t.Helper()
	cookies := w.Result().Cookies()
	for _, c := range cookies {
		if c.Name == name && c.Value != "" && c.MaxAge >= 0 {
			t.Errorf("unexpected cookie %q found with value %q", name, c.Value)
		}
	}
}

// Request Helpers

// NewJSONRequest creates a new HTTP request with JSON body
func NewJSONRequest(t *testing.T, method, url string, body interface{}) *http.Request {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		reader = strings.NewReader(string(data))
	}
	req := httptest.NewRequest(method, url, reader)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// NewRequestWithCookie creates a new HTTP request with a session cookie
func NewRequestWithCookie(t *testing.T, method, url, cookieName, cookieValue string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, url, nil)
	req.AddCookie(&http.Cookie{
		Name:  cookieName,
		Value: cookieValue,
	})
	return req
}

// DecodeJSON decodes JSON response body into the given struct
func DecodeJSON[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var result T
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode JSON response: %v. Body: %s", err, w.Body.String())
	}
	return result
}

// Slice Helpers

// AssertLen fails if the slice doesn't have the expected length
func AssertLen[T any](t *testing.T, slice []T, expected int) {
	t.Helper()
	if len(slice) != expected {
		t.Errorf("expected length %d, got %d", expected, len(slice))
	}
}

// AssertEmpty fails if the slice is not empty
func AssertEmpty[T any](t *testing.T, slice []T) {
	t.Helper()
	if len(slice) != 0 {
		t.Errorf("expected empty slice, got %d elements", len(slice))
	}
}

// AssertNotEmpty fails if the slice is empty
func AssertNotEmpty[T any](t *testing.T, slice []T) {
	t.Helper()
	if len(slice) == 0 {
		t.Error("expected non-empty slice, got empty")
	}
}
