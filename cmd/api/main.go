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
	"budgetapp/internal/repository"
	"budgetapp/internal/routes"
	"budgetapp/internal/service"

	"github.com/labstack/echo/v4"
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
	accountRepo := repository.NewAccountsRepository(pool)

	// Services (business logic)
	authService := service.NewAuthService(userRepo, tokens)
	budgetService := service.NewBudgetService(budgetRepo, categoryRepo)
	transactionService := service.NewTransactionService(transactionRepo, categoryRepo)
	summaryService := service.NewSummaryService(summaryRepo)
	accountService := service.NewAccountsService(accountRepo)

	// Handlers (HTTP)
	authHandler := handler.NewAuthHandler(authService)
	budgetHandler := handler.NewBudgetHandler(budgetService)
	transactionHandler := handler.NewTransactionHandler(transactionService)
	summaryHandler := handler.NewSummaryHandler(summaryService)
	accountHandler := handler.NewAccountsHandler(accountService)

	e := echo.New()
	e.HideBanner = true

	// Register middleware
	routes.RegisterMiddleware(e)

	// Register health check
	routes.RegisterHealthCheck(e)

	// Register Swagger/OpenAPI documentation
	routes.RegisterSwaggerUI(e)

	// Register public routes (no authentication required)
	routes.RegisterPublicRoutes(e, authHandler)

	// Register protected routes (authentication required)
	routes.RegisterProtectedRoutes(
		e,
		tokens,
		authHandler,
		budgetHandler,
		accountHandler,
		transactionHandler,
		summaryHandler,
	)

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
