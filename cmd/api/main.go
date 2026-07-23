// Command api is the composition root: it wires config -> database ->
// repositories -> services -> handlers -> routes, and nothing else in the
// codebase constructs these types. Everything downstream receives its
// dependencies through its constructor.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"budgetapp/internal/auth"
	"budgetapp/internal/config"
	"budgetapp/internal/database"
	"budgetapp/internal/handler"
	appmw "budgetapp/internal/middleware"
	"budgetapp/internal/repository"
	"budgetapp/internal/service"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	cfg := config.Load()

	pool, err := database.NewPool(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer pool.Close()

	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTExpiry)

	// Repositories (data access)
	userRepo := repository.NewUserRepository(pool)
	categoryRepo := repository.NewCategoryRepository(pool)
	budgetRepo := repository.NewBudgetRepository(pool)
	transactionRepo := repository.NewTransactionRepository(pool)
	summaryRepo := repository.NewSummaryRepository(pool)

	// Services (business logic)
	authService := service.NewAuthService(userRepo, tokens)
	budgetService := service.NewBudgetService(budgetRepo, categoryRepo)
	transactionService := service.NewTransactionService(transactionRepo, categoryRepo)
	summaryService := service.NewSummaryService(summaryRepo)

	// Handlers (HTTP)
	authHandler := handler.NewAuthHandler(authService)
	budgetHandler := handler.NewBudgetHandler(budgetService)
	transactionHandler := handler.NewTransactionHandler(transactionService)
	summaryHandler := handler.NewSummaryHandler(summaryService)

	e := echo.New()
	e.HideBanner = true

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

	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, echo.Map{"status": "ok"})
	})

	e.POST("/api/auth/register", authHandler.Register)
	e.POST("/api/auth/login", authHandler.Login)

	api := e.Group("/api", appmw.JWT(tokens))
	api.GET("/me", authHandler.Me)

	api.GET("/budgets", budgetHandler.List)
	api.POST("/budgets", budgetHandler.Create)
	api.PUT("/budgets/:id", budgetHandler.Update)
	api.DELETE("/budgets/:id", budgetHandler.Delete)

	api.GET("/transactions", transactionHandler.List)
	api.POST("/transactions", transactionHandler.Create)
	api.PUT("/transactions/:id", transactionHandler.Update)
	api.DELETE("/transactions/:id", transactionHandler.Delete)

	api.GET("/summary", summaryHandler.Get)

	// Run the server in a goroutine so we can listen for shutdown signals
	// and drain in-flight requests instead of killing connections abruptly.
	go func() {
		if err := e.Start(":" + cfg.Port); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		log.Fatalf("graceful shutdown failed: %v", err)
	}
}
