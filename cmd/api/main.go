// Command api is the composition root: it wires config -> database ->
// repositories -> services -> handlers -> routes, and nothing else in the
// codebase constructs these types. Everything downstream receives its
// dependencies through its constructor.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"budgetapp/internal/auth"
	"budgetapp/internal/config"
	"budgetapp/internal/database"
	"budgetapp/internal/handler"
	"budgetapp/internal/logger"
	"budgetapp/internal/repository"
	"budgetapp/internal/routes"
	"budgetapp/internal/service"

	"github.com/labstack/echo/v4"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

// run holds the real startup/shutdown logic so that deferred cleanup
// (pool.Close, stop) always executes, unlike with log.Fatal / os.Exit.
func run() error {
	// Configuration first: it tells us how to build the logger.
	cfg := config.Load()

	// Logger is built once here and injected everywhere.
	base := logger.New(cfg.LogLevel, cfg.LogFormat)
	slog.SetDefault(base)
	base.Info("starting app")

	// ctx is cancelled on Ctrl+C / SIGTERM and drives all shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Database pool
	pool, err := database.NewPool(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	defer pool.Close()

	// JWT auth tokens
	tokens := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTExpiry)

	// Repositories (data access)
	userRepo := repository.NewUserRepository(pool)
	categoryRepo := repository.NewCategoryRepository(pool)
	budgetRepo := repository.NewBudgetRepository(pool)
	transactionRepo := repository.NewTransactionRepository(pool)
	summaryRepo := repository.NewSummaryRepository(pool)
	accountRepo := repository.NewAccountsRepository(pool)
	ledgerRepo := repository.NewLedgerRepository(pool)
	balanceRepo := repository.NewAccountBalanceRepository(pool)
	goalRepo := repository.NewGoalRepository(pool)
	recurringRepo := repository.NewRecurringRepository(pool)

	// Services (business logic)
	authService := service.NewAuthService(userRepo, tokens, base)
	budgetService := service.NewBudgetService(budgetRepo, categoryRepo, base)
	categoryService := service.NewCategoryService(categoryRepo, base)
	transactionService := service.NewTransactionService(transactionRepo, categoryRepo, accountRepo, ledgerRepo, balanceRepo, pool, base)
	summaryService := service.NewSummaryService(summaryRepo, base)
	accountService := service.NewAccountsService(accountRepo, base)
	goalService := service.NewGoalService(goalRepo, base)
	recurringService := service.NewRecurringService(recurringRepo, categoryRepo, transactionService, base)

	// Handlers (HTTP)
	authHandler := handler.NewAuthHandler(authService, base)
	budgetHandler := handler.NewBudgetHandler(budgetService)
	categoryHandler := handler.NewCategoryHandler(categoryService)
	transactionHandler := handler.NewTransactionHandler(transactionService)
	summaryHandler := handler.NewSummaryHandler(summaryService)
	accountHandler := handler.NewAccountsHandler(accountService)
	goalHandler := handler.NewGoalHandler(goalService)
	recurringHandler := handler.NewRecurringHandler(recurringService)

	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = handler.HTTPErrorHandler

	routes.RegisterMiddleware(e)
	routes.RegisterHealthCheck(e)
	routes.RegisterSwaggerUI(e, base)
	routes.RegisterPublicRoutes(e, authHandler)
	routes.RegisterProtectedRoutes(
		e,
		tokens,
		base,
		authHandler,
		budgetHandler,
		accountHandler,
		categoryHandler,
		transactionHandler,
		summaryHandler,
		goalHandler,
		recurringHandler,
	)

	// In-process scheduler for recurring rules. Fires every hour and
	// materializes any rules whose next_run_at <= today. Safe to run
	// multiple replicas of the app — the recurring_rule_runs unique index
	// (rule_id, ran_for_date) prevents double-firing.
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				success, failed, err := recurringService.RunDue(ctx)
				if err != nil {
					base.ErrorContext(ctx, "recurring RunDue failed", "error", err)
					continue
				}
				if success > 0 || failed > 0 {
					base.InfoContext(ctx, "recurring RunDue finished", "success", success, "failed", failed)
				}
			}
		}
	}()

	// Run the server in a goroutine so we can wait for either a shutdown
	// signal or a server failure.
	serverErr := make(chan error, 1)
	go func() {
		if err := e.Start(":" + cfg.Port); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		base.Info("shutting down")
	case err := <-serverErr:
		return fmt.Errorf("server failed: %w", err)
	}

	// Drain in-flight requests; use a fresh context since ctx is cancelled.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}

	base.Info("shutdown complete")
	return nil
}
