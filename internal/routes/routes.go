// Package routes registers all API endpoints.
package routes

import (
	"net/http"
	"os"
	"path/filepath"

	"budgetapp/internal/auth"
	"budgetapp/internal/handler"
	appmw "budgetapp/internal/middleware"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// RegisterPublicRoutes registers all public (unauthenticated) API routes.
func RegisterPublicRoutes(
	e *echo.Echo,
	authHandler *handler.AuthHandler,
) {
	// Auth routes
	e.POST("/api/auth/register", authHandler.Register)
	e.POST("/api/auth/login", authHandler.Login)
	e.PUT("/api/password/reset", authHandler.ResetPassword)
	e.POST("/api/forgot-password", authHandler.ForgotPassword)
}

// RegisterProtectedRoutes registers all protected (authenticated) API routes.
// These routes require a valid JWT token in the Authorization header.
func RegisterProtectedRoutes(
	e *echo.Echo,
	tokens *auth.TokenManager,
	authHandler *handler.AuthHandler,
	budgetHandler *handler.BudgetHandler,
	accountHandler *handler.AccountHandler,
	categoryHandler *handler.CategoryHandler,
	transactionHandler *handler.TransactionHandler,
	summaryHandler *handler.SummaryHandler,
) {
	api := e.Group("/api", appmw.JWT(tokens))

	// User routes
	api.GET("/me", authHandler.Me)

	// Category routes
	api.GET("/categories", categoryHandler.List)

	// Budget routes
	api.GET("/budgets", budgetHandler.List)
	api.POST("/budgets", budgetHandler.Create)
	api.PUT("/budgets/:id", budgetHandler.Update)
	api.DELETE("/budgets/:id", budgetHandler.Delete)

	// Accounts APIs
	api.POST("/accounts", accountHandler.CreateAccount)
	api.GET("/accounts", accountHandler.ListAccounts)
	api.PUT("/accounts/:id", accountHandler.UpdateAccount)
	api.DELETE("/accounts/:id", accountHandler.DeleteAccount)

	// Transaction routes
	api.GET("/transactions", transactionHandler.List)
	api.GET("/transactions/:id", transactionHandler.GetByID)
	api.POST("/transactions", transactionHandler.Create)
	api.PUT("/transactions/:id", transactionHandler.Update)
	api.POST("/transactions/income", transactionHandler.CreateIncome)
	api.POST("/transactions/expense", transactionHandler.CreateExpense)
	api.POST("/transactions/transfer", transactionHandler.CreateTransfer)
	api.DELETE("/transactions/:id", transactionHandler.Delete)

	// Summary routes
	api.GET("/summary", summaryHandler.Get)
}

// RegisterMiddleware registers global middleware.
func RegisterMiddleware(e *echo.Echo) {
	e.Use(middleware.RequestID())
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOriginFunc: func(origin string) (bool, error) {
			return true, nil
		},
		AllowMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
		},
		AllowHeaders: []string{
			echo.HeaderContentType,
			echo.HeaderAuthorization,
		},
		AllowCredentials: true,
	}))
}

// RegisterHealthCheck registers the health check endpoint.
func RegisterHealthCheck(e *echo.Echo) {
	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, echo.Map{"status": "ok"})
	})
}

// RegisterSwaggerUI registers the Swagger/OpenAPI documentation endpoint.
func RegisterSwaggerUI(e *echo.Echo) {
	// Test route to verify function is called
	e.GET("/test-swagger-init", func(c echo.Context) error {
		return c.JSON(http.StatusOK, echo.Map{"message": "RegisterSwaggerUI was called"})
	})

	// Get the current working directory
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	docsPath := filepath.Join(wd, "docs")

	// Log to help debug
	e.Logger.Infof("Docs path: %s\n", docsPath)

	serveDocs := func(c echo.Context) error {
		indexPath := filepath.Join(docsPath, "index.html")
		c.Logger().Infof("Serving docs from: %s\n", indexPath)
		if _, err := os.Stat(indexPath); err != nil {
			c.Logger().Errorf("File not found: %s, error: %v\n", indexPath, err)
			return c.JSON(http.StatusNotFound, echo.Map{"error": "file not found", "path": indexPath})
		}
		return c.File(indexPath)
	}

	// Serve both documentation URL conventions.
	e.GET("/docs", serveDocs)
	e.GET("/docs/", serveDocs)
	e.GET("/api-docs", serveDocs)
	e.GET("/api-docs/", serveDocs)

	serveSwaggerSpec := func(c echo.Context) error {
		c.Response().Header().Set(echo.HeaderContentType, "application/x-yaml; charset=UTF-8")
		yamlPath := filepath.Join(docsPath, "swagger.yaml")
		return c.File(yamlPath)
	}

	e.GET("/docs/swagger.yaml", serveSwaggerSpec)
	e.GET("/api-docs/swagger.yaml", serveSwaggerSpec)
}
