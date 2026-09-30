// Package routes registers all API endpoints.
package routes

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"budgetapp/internal/auth"
	"budgetapp/internal/handler"
	appmw "budgetapp/internal/middleware"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func RegisterPublicRoutes(
	e *echo.Echo,
	authHandler *handler.AuthHandler,
) {
	log.Println("[routes] RegisterPublicRoutes: registering public routes")

	e.POST("/api/auth/register", authHandler.Register)
	e.POST("/api/auth/login", authHandler.Login)
	e.PUT("/api/password/reset", authHandler.ResetPassword)
	e.POST("/api/forgot-password", authHandler.ForgotPassword)

	log.Println("[routes] RegisterPublicRoutes: OK (4 routes)")
}

func RegisterProtectedRoutes(
	e *echo.Echo,
	tokens *auth.TokenManager,
	authHandler *handler.AuthHandler,
	budgetHandler *handler.BudgetHandler,
	accountHandler *handler.AccountHandler,
	categoryHandler *handler.CategoryHandler,
	transactionHandler *handler.TransactionHandler,
	summaryHandler *handler.SummaryHandler,
	goalHandler *handler.GoalHandler,
	recurringHandler *handler.RecurringHandler,
) {
	log.Println("[routes] RegisterProtectedRoutes: registering protected routes")

	api := e.Group("/api", appmw.JWT(tokens))

	api.GET("/me", authHandler.Me)

	api.GET("/categories", categoryHandler.List)

	// ---- budgets (per-month via ?month=YYYY-MM) ----
	api.GET("/budgets", budgetHandler.List)
	api.POST("/budgets", budgetHandler.Create)
	api.PUT("/budgets/:id", budgetHandler.Update)
	api.DELETE("/budgets/:id", budgetHandler.Delete)
	api.POST("/budgets/copy", budgetHandler.CopyFromPreviousMonth)

	// ---- accounts ----
	api.POST("/accounts", accountHandler.CreateAccount)
	api.GET("/accounts", accountHandler.ListAccounts)
	api.PUT("/accounts/:id", accountHandler.UpdateAccount)
	api.DELETE("/accounts/:id", accountHandler.DeleteAccount)

	// ---- transactions ----
	api.GET("/transactions", transactionHandler.List)
	api.GET("/transactions/:id", transactionHandler.GetByID)
	api.POST("/transactions", transactionHandler.Create)
	api.PUT("/transactions/:id", transactionHandler.Update)
	api.POST("/transactions/income", transactionHandler.CreateIncome)
	api.POST("/transactions/expense", transactionHandler.CreateExpense)
	api.POST("/transactions/transfer", transactionHandler.CreateTransfer)
	api.DELETE("/transactions/:id", transactionHandler.Delete)

	// ---- summary (per-month via ?month=YYYY-MM) ----
	api.GET("/summary", summaryHandler.Get)

	// ---- goals ----
	api.GET("/goals", goalHandler.List)
	api.POST("/goals", goalHandler.Create)
	api.PUT("/goals/:id", goalHandler.Update)
	api.POST("/goals/:id/contribute", goalHandler.Contribute)
	api.DELETE("/goals/:id", goalHandler.Delete)

	// ---- recurring ----
	api.GET("/recurring", recurringHandler.List)
	api.POST("/recurring", recurringHandler.Create)
	api.PUT("/recurring/:id", recurringHandler.Update)
	api.POST("/recurring/:id/pause", recurringHandler.SetActive(false))
	api.POST("/recurring/:id/resume", recurringHandler.SetActive(true))
	api.DELETE("/recurring/:id", recurringHandler.Delete)
	api.POST("/recurring/run", recurringHandler.RunDue)

	log.Println("[routes] RegisterProtectedRoutes: OK (33 routes)")
}

func RegisterMiddleware(e *echo.Echo) {
	log.Println("[routes] RegisterMiddleware: registering global middleware")

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
			"Idempotency-Key",
		},
		AllowCredentials: true,
	}))

	log.Println("[routes] RegisterMiddleware: OK (RequestID, Logger, Recover, CORS)")
}

func RegisterHealthCheck(e *echo.Echo) {
	log.Println("[routes] RegisterHealthCheck: registering /healthz")

	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, echo.Map{"status": "ok"})
	})
}

func RegisterSwaggerUI(e *echo.Echo) {
	log.Println("[routes] RegisterSwaggerUI: registering swagger docs routes")

	e.GET("/test-swagger-init", func(c echo.Context) error {
		return c.JSON(http.StatusOK, echo.Map{"message": "RegisterSwaggerUI was called"})
	})

	wd, err := os.Getwd()
	if err != nil {
		log.Printf("[routes] RegisterSwaggerUI: failed to get working directory: %v", err)
		panic(err)
	}

	docsPath := filepath.Join(wd, "docs")
	log.Printf("[routes] RegisterSwaggerUI: docs path=%s", docsPath)

	serveDocs := func(c echo.Context) error {
		indexPath := filepath.Join(docsPath, "index.html")
		log.Printf("[routes] RegisterSwaggerUI: serving docs from=%s", indexPath)
		if _, err := os.Stat(indexPath); err != nil {
			log.Printf("[routes] RegisterSwaggerUI: file not found path=%s error=%v", indexPath, err)
			return c.JSON(http.StatusNotFound, echo.Map{"error": "file not found", "path": indexPath})
		}
		return c.File(indexPath)
	}

	e.GET("/docs", serveDocs)
	e.GET("/docs/", serveDocs)
	e.GET("/api-docs", serveDocs)
	e.GET("/api-docs/", serveDocs)

	serveSwaggerSpec := func(c echo.Context) error {
		c.Response().Header().Set(echo.HeaderContentType, "application/x-yaml; charset=UTF-8")
		yamlPath := filepath.Join(docsPath, "swagger.yaml")
		log.Printf("[routes] RegisterSwaggerUI: serving swagger spec from=%s", yamlPath)
		return c.File(yamlPath)
	}

	e.GET("/docs/swagger.yaml", serveSwaggerSpec)
	e.GET("/api-docs/swagger.yaml", serveSwaggerSpec)

	log.Println("[routes] RegisterSwaggerUI: OK")
}
