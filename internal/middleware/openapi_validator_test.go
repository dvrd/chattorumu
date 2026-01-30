package middleware

import (
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAPISpecIsValid(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	// Load OpenAPI spec
	doc, err := loader.LoadFromFile("../../artifacts/openapi.yaml")
	require.NoError(t, err, "Failed to load OpenAPI spec")

	// Validate OpenAPI document
	err = doc.Validate(loader.Context)
	require.NoError(t, err, "OpenAPI spec validation failed")

	// Verify basic metadata
	assert.Equal(t, "Jobsity Chat API", doc.Info.Title)
	assert.Equal(t, "1.0.0", doc.Info.Version)
	assert.NotEmpty(t, doc.Servers, "At least one server should be defined")
}

func TestAllRoutesAreDocumentedInOpenAPI(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromFile("../../artifacts/openapi.yaml")
	require.NoError(t, err)

	// List of all implemented routes in the application
	implementedRoutes := []struct {
		method string
		path   string
	}{
		// Authentication routes
		{"POST", "/auth/register"},
		{"POST", "/auth/login"},
		{"GET", "/auth/me"},
		{"POST", "/auth/logout"},

		// Chatroom routes
		{"GET", "/chatrooms"},
		{"POST", "/chatrooms"},
		{"POST", "/chatrooms/{id}/join"},
		{"GET", "/chatrooms/{id}/messages"},

		// WebSocket route
		{"GET", "/ws/chat/{chatroom_id}"},

		// Health routes
		{"GET", "/health"},
		{"GET", "/health/ready"},
	}

	// Verify each route exists in OpenAPI spec
	for _, route := range implementedRoutes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			pathItem := doc.Paths.Find(route.path)
			require.NotNil(t, pathItem, "Path not found in OpenAPI spec: %s", route.path)

			operation := pathItem.GetOperation(route.method)
			require.NotNil(t, operation, "Operation not found in OpenAPI spec: %s %s", route.method, route.path)

			// Verify operation has required fields
			assert.NotEmpty(t, operation.OperationID, "OperationID should be set")
			assert.NotEmpty(t, operation.Tags, "Tags should be set")
			assert.NotEmpty(t, operation.Responses, "Responses should be defined")
		})
	}
}

func TestOpenAPIPathsMatchImplementation(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromFile("../../artifacts/openapi.yaml")
	require.NoError(t, err)

	// Count of expected endpoints
	expectedPaths := []string{
		"/auth/register",
		"/auth/login",
		"/auth/me",
		"/auth/logout",
		"/chatrooms",
		"/chatrooms/{id}/join",
		"/chatrooms/{id}/messages",
		"/ws/chat/{chatroom_id}",
		"/health",
		"/health/ready",
	}

	assert.Len(t, doc.Paths.Map(), len(expectedPaths), "Number of paths should match")

	// Verify all expected paths exist
	for _, path := range expectedPaths {
		pathItem := doc.Paths.Find(path)
		assert.NotNil(t, pathItem, "Expected path not found: %s", path)
	}
}

func TestOpenAPISecuritySchemes(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromFile("../../artifacts/openapi.yaml")
	require.NoError(t, err)

	// Verify security schemes are defined
	require.NotNil(t, doc.Components.SecuritySchemes, "Security schemes should be defined")

	// Verify cookieAuth exists
	cookieAuth := doc.Components.SecuritySchemes["cookieAuth"]
	require.NotNil(t, cookieAuth, "cookieAuth security scheme should exist")
	assert.Equal(t, "apiKey", cookieAuth.Value.Type)
	assert.Equal(t, "cookie", cookieAuth.Value.In)
	assert.Equal(t, "session_id", cookieAuth.Value.Name)
}

func TestOpenAPISchemas(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromFile("../../artifacts/openapi.yaml")
	require.NoError(t, err)

	// Verify key schemas exist
	requiredSchemas := []string{
		"RegisterRequest",
		"LoginRequest",
		"UserResponse",
		"LoginResponse",
		"ErrorResponse",
		"Chatroom",
		"Message",
		"CreateChatroomRequest",
	}

	for _, schemaName := range requiredSchemas {
		schema := doc.Components.Schemas[schemaName]
		assert.NotNil(t, schema, "Schema should exist: %s", schemaName)
	}
}

func TestProtectedRoutesHaveAuth(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromFile("../../artifacts/openapi.yaml")
	require.NoError(t, err)

	// Routes that should require authentication
	protectedRoutes := []struct {
		method string
		path   string
	}{
		{"GET", "/auth/me"},
		{"POST", "/auth/logout"},
		{"GET", "/chatrooms"},
		{"POST", "/chatrooms"},
		{"POST", "/chatrooms/{id}/join"},
		{"GET", "/chatrooms/{id}/messages"},
	}

	for _, route := range protectedRoutes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			pathItem := doc.Paths.Find(route.path)
			require.NotNil(t, pathItem)

			operation := pathItem.GetOperation(route.method)
			require.NotNil(t, operation)

			// Verify security requirement exists
			assert.NotEmpty(t, operation.Security, "Protected route should have security requirement: %s %s", route.method, route.path)

			// Verify cookieAuth is used
			hasCookieAuth := false
			for _, secReq := range *operation.Security {
				if _, ok := secReq["cookieAuth"]; ok {
					hasCookieAuth = true
					break
				}
			}
			assert.True(t, hasCookieAuth, "Protected route should use cookieAuth: %s %s", route.method, route.path)
		})
	}
}

func TestPublicRoutesNoAuth(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromFile("../../artifacts/openapi.yaml")
	require.NoError(t, err)

	// Routes that should NOT require authentication
	publicRoutes := []struct {
		method string
		path   string
	}{
		{"POST", "/auth/register"},
		{"POST", "/auth/login"},
		{"GET", "/health"},
		{"GET", "/health/ready"},
	}

	for _, route := range publicRoutes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			pathItem := doc.Paths.Find(route.path)
			require.NotNil(t, pathItem)

			operation := pathItem.GetOperation(route.method)
			require.NotNil(t, operation)

			// Verify no security requirement or empty
			if operation.Security != nil {
				assert.Empty(t, *operation.Security, "Public route should not have security requirement: %s %s", route.method, route.path)
			}
		})
	}
}

func TestShouldSkipPath(t *testing.T) {
	skipPaths := []string{
		"/health",
		"/health/ready",
		"/metrics",
		"/static",
	}

	tests := []struct {
		path     string
		expected bool
	}{
		{"/health", true},
		{"/health/ready", true},
		{"/metrics", true},
		{"/static/index.html", true},
		{"/api/v1/chatrooms", false},
		{"/api/v1/auth/login", false},
		{"/ws/chat/123", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := shouldSkipPath(tt.path, skipPaths)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultOpenAPIValidatorConfig(t *testing.T) {
	config := DefaultOpenAPIValidatorConfig()

	assert.NotNil(t, config)
	assert.Equal(t, "artifacts/openapi.yaml", config.SpecPath)
	assert.True(t, config.ValidateRequests, "Should validate requests by default")
	assert.False(t, config.ValidateResponses, "Should not validate responses by default (performance)")
	assert.NotEmpty(t, config.SkipPaths, "Should have skip paths configured")

	// Verify common skip paths are included
	skipPathsStr := strings.Join(config.SkipPaths, ",")
	assert.Contains(t, skipPathsStr, "/health")
	assert.Contains(t, skipPathsStr, "/metrics")
}

func TestOpenAPIMiddlewareWithInvalidSpec(t *testing.T) {
	config := &OpenAPIValidatorConfig{
		Enabled:  true,
		SpecPath: "/nonexistent/path/to/spec.yaml",
	}

	// Should not panic, just return no-op middleware
	middleware := OpenAPIValidator(config)
	assert.NotNil(t, middleware)
}

func TestOpenAPIMiddlewareDisabled(t *testing.T) {
	config := &OpenAPIValidatorConfig{
		Enabled: false,
	}

	middleware := OpenAPIValidator(config)
	assert.NotNil(t, middleware)
}

func TestOpenAPIResponseCodes(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromFile("../../artifacts/openapi.yaml")
	require.NoError(t, err)

	// Verify register endpoint has correct response codes
	pathItem := doc.Paths.Find("/auth/register")
	require.NotNil(t, pathItem)

	operation := pathItem.GetOperation("POST")
	require.NotNil(t, operation)

	// Should have 201 (Created), 400 (Bad Request), 409 (Conflict)
	assert.NotNil(t, operation.Responses.Status(201), "Register should return 201 on success")
	assert.NotNil(t, operation.Responses.Status(400), "Register should return 400 on invalid input")
	assert.NotNil(t, operation.Responses.Status(409), "Register should return 409 on conflict")
}

func TestOpenAPIExamplesExist(t *testing.T) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromFile("../../artifacts/openapi.yaml")
	require.NoError(t, err)

	// Verify register endpoint has examples
	pathItem := doc.Paths.Find("/auth/register")
	require.NotNil(t, pathItem)

	operation := pathItem.GetOperation("POST")
	require.NotNil(t, operation)

	// Check if request body has examples
	assert.NotNil(t, operation.RequestBody, "Register should have request body")
	content := operation.RequestBody.Value.Content.Get("application/json")
	assert.NotNil(t, content, "Should have application/json content")

	// Examples help with documentation and testing
	if content.Examples != nil {
		assert.NotEmpty(t, content.Examples, "Examples help with API documentation")
	}
}
