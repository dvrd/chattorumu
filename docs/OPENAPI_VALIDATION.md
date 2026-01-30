# OpenAPI Validation Middleware

## Overview

The OpenAPI validation middleware automatically validates all HTTP requests and responses against the OpenAPI specification (`artifacts/openapi.yaml`). This ensures that your implementation stays in sync with your API contract.

## Features

- ✅ **Request Validation**: Validates HTTP method, path parameters, query parameters, headers, and request body
- ✅ **Response Validation**: Validates HTTP status codes, headers, and response body (optional, disabled by default for performance)
- ✅ **Auto-Configuration**: Automatically enables in development, disables in production
- ✅ **Path Skipping**: Skip validation for health checks, metrics, and static files
- ✅ **Detailed Logging**: Uses `slog` for structured logging of validation errors
- ✅ **Non-Breaking**: If OpenAPI spec fails to load, middleware becomes no-op (doesn't break your app)

## Quick Start

The middleware is already integrated in `cmd/chat-server/main.go`:

```go
// Global middleware
r.Use(middleware.OpenAPIValidator(middleware.DefaultOpenAPIValidatorConfig()))
```

### Default Configuration

By default, the middleware:
- ✅ **Enabled in development** (`ENVIRONMENT != "production"`)
- ❌ **Disabled in production** (for performance)
- ✅ **Validates requests** (catches bugs early)
- ❌ **Skips response validation** (performance optimization)
- Skips paths: `/health`, `/health/ready`, `/metrics`, static files

## Configuration Options

### Custom Configuration

You can customize the middleware behavior:

```go
// Example: Enable in production with custom settings
config := &middleware.OpenAPIValidatorConfig{
    Enabled:           true,                      // Force enable
    SpecPath:          "artifacts/openapi.yaml",  // Path to OpenAPI spec
    ValidateRequests:  true,                      // Validate incoming requests
    ValidateResponses: false,                     // Validate outgoing responses (slow!)
    SkipPaths: []string{
        "/health",
        "/metrics",
        "/static",
    },
}

r.Use(middleware.OpenAPIValidator(config))
```

### Environment-Based Configuration

Control validation via environment variables:

```bash
# Development (validation enabled)
ENVIRONMENT=development go run ./cmd/chat-server

# Production (validation disabled)
ENVIRONMENT=production go run ./cmd/chat-server
```

## Validation Behavior

### Request Validation

When a request fails validation, the middleware returns:

```json
{
  "error": "Request validation failed: parameter 'id' in path is required"
}
```

HTTP Status: `400 Bad Request`

### Response Validation

When enabled, response validation logs warnings but **does not block** the response:

```
WARN response validation failed method=GET path=/api/v1/chatrooms status=200 error=response body doesn't match schema
```

> **Note**: Response validation impacts performance as it captures and parses response bodies. Use in development only.

### Path Not Found

If a request matches no OpenAPI path:

```json
{
  "error": "Path not found in OpenAPI spec: GET /api/v1/unknown"
}
```

HTTP Status: `400 Bad Request`

## Example Validation Errors

### Missing Required Field

**Request:**
```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username": "john"}'
```

**Response:**
```json
{
  "error": "Request validation failed: property 'email' is required"
}
```

### Invalid Type

**Request:**
```bash
curl -X POST http://localhost:8080/api/v1/chatrooms \
  -H "Content-Type: application/json" \
  -d '{"name": 123}'
```

**Response:**
```json
{
  "error": "Request validation failed: property 'name' must be string"
}
```

## Performance Considerations

### Request Validation
- **Overhead**: ~1-2ms per request
- **Recommendation**: Enable in development and staging
- **Production**: Disable or use sampling (validate 1% of requests)

### Response Validation
- **Overhead**: ~5-10ms per request (captures entire response body)
- **Recommendation**: Only enable in development for debugging
- **Production**: Always disable

## Debugging

### Enable Debug Logging

Set log level to debug to see validation details:

```go
// In main.go
slog.SetLogLoggerLevel(slog.LevelDebug)
```

You'll see:
```
DEBUG request validated successfully method=POST path=/api/v1/auth/login
DEBUG response validated successfully method=POST path=/api/v1/auth/login status=200
```

### Disable Validation Temporarily

```go
// Quick disable for testing
config := middleware.DefaultOpenAPIValidatorConfig()
config.Enabled = false
r.Use(middleware.OpenAPIValidator(config))
```

## CI/CD Integration

### Pre-Commit Hook

Validate OpenAPI spec before committing:

```bash
# .git/hooks/pre-commit
#!/bin/bash
go run github.com/getkin/kin-openapi/cmd/validate@latest artifacts/openapi.yaml
```

### Test Suite

Add validation tests:

```go
func TestOpenAPISpecIsValid(t *testing.T) {
    loader := openapi3.NewLoader()
    doc, err := loader.LoadFromFile("../../artifacts/openapi.yaml")
    require.NoError(t, err)

    err = doc.Validate(loader.Context)
    require.NoError(t, err)
}

func TestAllRoutesMatchOpenAPI(t *testing.T) {
    // Load OpenAPI spec
    loader := openapi3.NewLoader()
    doc, err := loader.LoadFromFile("../../artifacts/openapi.yaml")
    require.NoError(t, err)

    // Verify all implemented routes exist in spec
    implementedRoutes := []string{
        "POST /api/v1/auth/register",
        "POST /api/v1/auth/login",
        "GET /api/v1/auth/me",
        "POST /api/v1/auth/logout",
        // ... etc
    }

    for _, route := range implementedRoutes {
        parts := strings.Split(route, " ")
        method, path := parts[0], parts[1]

        pathItem := doc.Paths.Find(path)
        require.NotNil(t, pathItem, "Path not found in OpenAPI: %s", path)

        operation := pathItem.GetOperation(method)
        require.NotNil(t, operation, "Operation not found in OpenAPI: %s", route)
    }
}
```

## Troubleshooting

### "Failed to load OpenAPI spec"

**Cause**: `artifacts/openapi.yaml` not found or invalid YAML

**Solution**:
1. Verify file exists: `ls artifacts/openapi.yaml`
2. Validate YAML syntax: `yamllint artifacts/openapi.yaml`
3. Check working directory: middleware looks for `artifacts/openapi.yaml` relative to CWD

### "Path not found in OpenAPI spec"

**Cause**: Endpoint implemented but not documented in OpenAPI

**Solution**:
1. Add the endpoint to `artifacts/openapi.yaml`
2. Or add to `SkipPaths` if intentional (e.g., debug endpoints)

### Validation Too Strict

**Cause**: OpenAPI spec has strict validation rules

**Solution**:
1. Relax schema constraints (e.g., make fields optional)
2. Add `additionalProperties: true` to allow extra fields
3. Use `oneOf`/`anyOf` for flexible schemas

## Best Practices

1. ✅ **Keep OpenAPI spec in sync**: Update spec when adding/changing endpoints
2. ✅ **Use validation in dev**: Catch contract violations early
3. ✅ **Disable in prod**: Optimize for performance
4. ✅ **Skip non-API paths**: Don't validate health checks, metrics, static files
5. ✅ **Test your spec**: Add tests that validate the OpenAPI spec itself
6. ✅ **Use examples**: Add examples to OpenAPI spec for better documentation
7. ❌ **Don't validate responses in prod**: Too slow, use sampling instead

## Related Tools

- **Swagger UI**: Interactive API documentation
  ```bash
  docker run -p 8081:8080 -e SWAGGER_JSON=/api/openapi.yaml -v $(pwd)/artifacts:/api swaggerapi/swagger-ui
  ```

- **ReDoc**: Beautiful API documentation
  ```bash
  npx @redocly/cli preview-docs artifacts/openapi.yaml
  ```

- **Spectral**: OpenAPI linter
  ```bash
  npx @stoplight/spectral-cli lint artifacts/openapi.yaml
  ```

- **oapi-codegen**: Generate Go code from OpenAPI
  ```bash
  go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
  oapi-codegen -package api artifacts/openapi.yaml > internal/api/openapi_gen.go
  ```

## Summary

The OpenAPI validation middleware helps maintain API contract compliance automatically. It's enabled by default in development and catches bugs before they reach production.

**Current Status:**
- ✅ Middleware implemented: `internal/middleware/openapi_validator.go`
- ✅ Integrated in server: `cmd/chat-server/main.go:113`
- ✅ OpenAPI spec complete: `artifacts/openapi.yaml` (10 endpoints)
- ✅ All endpoints documented including `/auth/me`

**Next Steps:**
1. Run server in dev mode and verify validation works
2. Add OpenAPI validation tests to test suite
3. Consider adding Swagger UI for interactive documentation
4. Set up CI/CD linting with Spectral
