# API Documentation Guide

## Overview
The BudgetFlow API now includes comprehensive Swagger/OpenAPI documentation with organized route files.

## What's New

### 1. Routes Separation
All API routes have been extracted into a dedicated routes package at `internal/routes/routes.go`. This provides:
- **Clear separation of concerns**: Route registration is separate from main.go
- **Easy maintenance**: Add or modify routes in one central location
- **Better organization**: Public, protected, and utility routes are clearly separated

### 2. OpenAPI/Swagger Documentation
A complete OpenAPI 3.0 specification has been created at `openapi.yaml` with:
- **All endpoints documented**: 14+ endpoints with full descriptions
- **Request/response schemas**: Clear JSON schemas for all inputs and outputs
- **Error handling**: Documented error responses with HTTP status codes
- **Authentication**: JWT bearer token requirements documented
- **Tags and organization**: Endpoints grouped by feature (Auth, Budgets, Transactions, etc.)

### 3. Documentation Endpoint
Access the interactive API documentation at:
```
http://localhost:8080/api-docs
```

## Accessing the API Documentation

### Interactive Swagger UI
1. Start your server: `go run cmd/api/main.go`
2. Visit: `http://localhost:8080/api-docs`
3. The interface shows all endpoints with descriptions and allows you to test them directly

### OpenAPI Specification Files
- **YAML format**: `http://localhost:8080/api-docs/swagger.yaml`
- **JSON format**: `http://localhost:8080/api-docs/openapi.json`

These can be used with:
- [Swagger UI](https://swagger.io/tools/swagger-ui/)
- [Redoc](https://redoc.ly/) (currently used in /api-docs)
- [Postman](https://www.postman.com/)
- Any OpenAPI-compatible tool

## Route Organization

### Public Routes (No Authentication Required)
Located in `RegisterPublicRoutes()`:
- `POST /api/auth/register` - Create new user account
- `POST /api/auth/login` - Authenticate and get JWT token
- `POST /api/forgot-password` - Request password reset
- `PUT /api/password/reset` - Reset password with token

### Protected Routes (JWT Authentication Required)
Located in `RegisterProtectedRoutes()`:

**User:**
- `GET /api/me` - Get current user profile

**Budgets:**
- `GET /api/budgets` - List all budgets
- `POST /api/budgets` - Create new budget
- `PUT /api/budgets/:id` - Update budget
- `DELETE /api/budgets/:id` - Delete budget

**Transactions:**
- `GET /api/transactions` - List all transactions
- `POST /api/transactions` - Create transaction
- `PUT /api/transactions/:id` - Update transaction
- `DELETE /api/transactions/:id` - Delete transaction

**Summary:**
- `GET /api/summary` - Get financial summary

## Using the Swagger UI

### Testing Endpoints
1. Navigate to `http://localhost:8080/api-docs`
2. Expand an endpoint section to see details
3. Click "Try it out" to test an endpoint
4. For protected endpoints, click the lock icon and add your JWT token
5. Fill in parameters and request body
6. Click "Execute" to send the request

### Getting Authentication Token
1. Use the `POST /api/auth/login` endpoint
2. Provide email and password
3. Copy the returned `token` value
4. Click the lock icon in the UI and paste the token

## Code Structure

```
cmd/api/main.go                 # Main entry point (cleaner now)
internal/
  ├── routes/
  │   └── routes.go            # All route registration logic
  ├── handler/                 # HTTP handlers
  ├── service/                 # Business logic
  ├── repository/              # Data access
  └── models/                  # Data models
openapi.yaml                   # OpenAPI specification
```

## Benefits of This Structure

1. **Scalability**: Easy to add new routes and features
2. **Documentation**: OpenAPI spec auto-serves as API documentation
3. **Testing**: External tools can generate client SDKs from the OpenAPI spec
4. **Maintainability**: Changes to routes are centralized
5. **DevOps**: The openapi.yaml can be used by API gateways and service mesh tools

## Next Steps

### Optional Enhancements
1. **Add request validation examples**: Update openapi.yaml with more detailed examples
2. **Generate client SDKs**: Use OpenAPI generator to create client libraries
3. **API versioning**: Prefix routes with `/api/v1/` to support versioning
4. **Rate limiting**: Add rate limiting middleware to routes.go
5. **Request tracing**: Add correlation IDs or tracing middleware

### Local Testing
```bash
# Start the server
go run cmd/api/main.go

# Health check
curl http://localhost:8080/healthz

# View API docs
open http://localhost:8080/api-docs
```

## Troubleshooting

### Swagger UI not loading
- Ensure `openapi.yaml` is in the project root
- Check that the server is running
- Verify the route is registered with `routes.RegisterSwaggerUI(e)`

### Authentication errors in docs
- Get a valid token from `/api/auth/login`
- Click the lock icon in Swagger UI
- Paste the token exactly as provided (without "Bearer " prefix, the UI adds it)

## References
- [OpenAPI 3.0 Specification](https://spec.openapis.org/oas/v3.0.3)
- [Swagger UI Documentation](https://swagger.io/tools/swagger-ui/)
- [Echo Framework Documentation](https://echo.labstack.com/)
