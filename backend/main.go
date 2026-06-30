package main

import (
	"budgetapp/db"
	"budgetapp/handlers"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	// Init DB
	database := db.Init()
	defer database.Close()

	e := echo.New()
	e.HideBanner = true

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"http://localhost:5173"},
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

	h := handlers.NewHandler(database)

	// Auth routes
	e.POST("/api/auth/register", h.Register)
	e.POST("/api/auth/login", h.Login)

	// Protected routes
	api := e.Group("/api", h.JWTMiddleware)
	api.GET("/me", h.GetMe)

	// Budgets
	api.GET("/budgets", h.GetBudgets)
	api.POST("/budgets", h.CreateBudget)
	api.PUT("/budgets/:id", h.UpdateBudget)
	api.DELETE("/budgets/:id", h.DeleteBudget)

	// Transactions
	api.GET("/transactions", h.GetTransactions)
	api.POST("/transactions", h.CreateTransaction)
	api.PUT("/transactions/:id", h.UpdateTransaction)
	api.DELETE("/transactions/:id", h.DeleteTransaction)

	// Summary
	api.GET("/summary", h.GetSummary)

	e.Logger.Fatal(e.Start(":8080"))
}
