package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"budgetapp/internal/auth"
	"budgetapp/internal/models"
	"budgetapp/internal/repository"
	"budgetapp/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load("/home/mashallah/Desktop/BudgetFlow-Backend/.env")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("no DATABASE_URL")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect failed: %v", err)
	}
	defer pool.Close()

	userRepo := repository.NewUserRepository(pool)
	catRepo := repository.NewCategoryRepository(pool)
	budgetRepo := repository.NewBudgetRepository(pool)
	txnRepo := repository.NewTransactionRepository(pool)
	summaryRepo := repository.NewSummaryRepository(pool)
	accountRepo := repository.NewAccountsRepository(pool)
	ledgerRepo := repository.NewLedgerRepository(pool)
	balanceRepo := repository.NewAccountBalanceRepository(pool)

	tokens := auth.NewTokenManager(os.Getenv("JWT_SECRET"), 24)
	authService := service.NewAuthService(userRepo, tokens)
	budgetService := service.NewBudgetService(budgetRepo, catRepo)
	txnService := service.NewTransactionService(txnRepo, catRepo, accountRepo, ledgerRepo, balanceRepo, pool)
	summaryService := service.NewSummaryService(summaryRepo)
	accountService := service.NewAccountsService(accountRepo)

	// 1. Create a test user
	testEmail := fmt.Sprintf("test_%d@example.com", time.Now().UnixNano())
	user, _, err := authService.Register(ctx, testEmail, "Password123!", "Test User")
	if err != nil {
		log.Fatalf("register failed: %v", err)
	}
	fmt.Printf("1. User created: ID=%d, Email=%s\n", user.ID, user.Email)

	// Clean up user at end
	defer func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
		fmt.Println("Cleaned up test user.")
	}()

	// 2. Create account
	acc, err := accountService.Create(ctx, user.ID, models.AccountInput{
		Name:     "M-Pesa Test",
		Type:     "mobile_money",
		Currency: "KES",
	})
	if err != nil {
		log.Fatalf("create account failed: %v", err)
	}
	fmt.Printf("2. Account created: ID=%d, Name=%s\n", acc.ID, acc.Name)

	// 3. Create budget with category "Dining"
	budget, err := budgetService.Create(ctx, user.ID, models.BudgetInput{
		Name:     "Monthly Dining",
		Amount:   15000,
		Category: "Dining",
		Color:    "#ef4444",
	})
	if err != nil {
		log.Fatalf("create budget failed: %v", err)
	}
	fmt.Printf("3. Budget created: ID=%d, Name=%s, Category=%s, Amount=%.2f\n", budget.ID, budget.Name, budget.Category, budget.Amount)

	// 4. Create Income transaction (+50,000)
	inc, err := txnService.CreateIncome(ctx, user.ID, models.TransactionInput{
		Title:     "Salary September",
		Amount:    50000,
		Type:      "income",
		Category:  "Salary",
		AccountID: acc.ID,
		Date:      time.Now().Format("2006-01-02"),
		Note:      "Monthly paycheck",
	})
	if err != nil {
		log.Fatalf("create income failed: %v", err)
	}
	fmt.Printf("4. Income created: ID=%d, Title=%s\n", inc.Transaction.ID, inc.Transaction.Title)

	// 5. Create Expense transaction (-3,500 on "Dining")
	exp, err := txnService.CreateExpense(ctx, user.ID, models.TransactionInput{
		Title:     "Dinner out",
		Amount:    3500,
		Type:      "expense",
		Category:  "Dining",
		AccountID: acc.ID,
		Date:      time.Now().Format("2006-01-02"),
		Note:      "Italian restaurant",
	})
	if err != nil {
		log.Fatalf("create expense failed: %v", err)
	}
	fmt.Printf("5. Expense created: ID=%d, Title=%s\n", exp.Transaction.ID, exp.Transaction.Title)

	// 6. Check Summary
	summary, err := summaryService.Get(ctx, user.ID)
	if err != nil {
		log.Fatalf("summary failed: %v", err)
	}
	fmt.Printf("6. Summary: Income=%.2f, Expense=%.2f, Balance=%.2f\n", summary.TotalIncome, summary.TotalExpenses, summary.Balance)
	if summary.TotalIncome != 50000 {
		log.Fatalf("FAIL: expected TotalIncome=50000, got %.2f", summary.TotalIncome)
	}
	if summary.TotalExpenses != 3500 {
		log.Fatalf("FAIL: expected TotalExpenses=3500, got %.2f", summary.TotalExpenses)
	}
	if summary.Balance != 46500 {
		log.Fatalf("FAIL: expected Balance=46500, got %.2f", summary.Balance)
	}
	if len(summary.BudgetStats) != 1 || summary.BudgetStats[0].Spent != 3500 {
		log.Fatalf("FAIL: expected BudgetStats[0].Spent=3500, got %+v", summary.BudgetStats)
	}
	fmt.Printf("   BudgetStat: Name=%s, Amount=%.2f, Spent=%.2f\n", summary.BudgetStats[0].Name, summary.BudgetStats[0].Amount, summary.BudgetStats[0].Spent)
	if len(summary.MonthlyData) == 0 || summary.MonthlyData[0].Expense != 3500 {
		log.Fatalf("FAIL: expected MonthlyData Expense=3500, got %+v", summary.MonthlyData)
	}
	fmt.Printf("   MonthlyData: Month=%s, Income=%.2f, Expense=%.2f\n", summary.MonthlyData[0].Month, summary.MonthlyData[0].Income, summary.MonthlyData[0].Expense)

	// 7. Check Budget List
	budgets, err := budgetService.List(ctx, user.ID)
	if err != nil {
		log.Fatalf("budget list failed: %v", err)
	}
	if len(budgets) != 1 || budgets[0].Spent != 3500 {
		log.Fatalf("FAIL: expected budgets[0].Spent=3500, got %.2f", budgets[0].Spent)
	}
	fmt.Printf("7. Budget list verified: Name=%s, Spent=%.2f, Category=%s\n", budgets[0].Name, budgets[0].Spent, budgets[0].Category)

	// 8. Check Enriched Transaction List
	txns, err := txnService.List(ctx, user.ID, models.TransactionFilter{})
	if err != nil {
		log.Fatalf("transaction list failed: %v", err)
	}
	if len(txns) != 2 {
		log.Fatalf("FAIL: expected 2 transactions, got %d", len(txns))
	}
	for _, t := range txns {
		fmt.Printf("8. Txn: ID=%d, Type=%s, Title=%s, Amount=%.2f, Category=%s, AccountName=%s\n",
			t.ID, t.Type, t.Title, t.Amount, t.Category, t.AccountName)
		if t.Amount <= 0 {
			log.Fatalf("FAIL: transaction %d has non-positive amount %.2f", t.ID, t.Amount)
		}
		if t.AccountName == "" {
			log.Fatalf("FAIL: transaction %d has empty AccountName", t.ID)
		}
	}

	// 9. Test Filter by Type=expense
	expenseTxns, err := txnService.List(ctx, user.ID, models.TransactionFilter{Type: "expense"})
	if err != nil {
		log.Fatalf("filtered list failed: %v", err)
	}
	if len(expenseTxns) != 1 || expenseTxns[0].Type != "expense" {
		log.Fatalf("FAIL: expected 1 expense transaction, got %d", len(expenseTxns))
	}
	fmt.Println("9. Filter by type=expense verified.")

	// 10. Test Update Transaction
	updated, err := txnService.Update(ctx, exp.Transaction.ID, user.ID, models.UpdateTransactionInput{
		Title: "Fine Italian Dining",
	})
	if err != nil {
		log.Fatalf("update txn failed: %v", err)
	}
	if updated.Transaction.Title != "Fine Italian Dining" {
		log.Fatalf("FAIL: expected title 'Fine Italian Dining', got '%s'", updated.Transaction.Title)
	}
	fmt.Printf("10. Transaction update verified: New Title=%s\n", updated.Transaction.Title)

	// 11. Test Categories List
	catService := service.NewCategoryService(catRepo)
	categories, err := catService.List(ctx, user.ID)
	if err != nil {
		log.Fatalf("cat list failed: %v", err)
	}
	fmt.Printf("11. Categories listed: %d categories found\n", len(categories))

	// 12. Test Delete
	err = txnService.Delete(ctx, exp.Transaction.ID, user.ID)
	if err != nil {
		log.Fatalf("delete txn failed: %v", err)
	}
	postDeleteSummary, err := summaryService.Get(ctx, user.ID)
	if err != nil {
		log.Fatalf("post-delete summary failed: %v", err)
	}
	if postDeleteSummary.TotalExpenses != 0 {
		log.Fatalf("FAIL: expected TotalExpenses=0 after delete, got %.2f", postDeleteSummary.TotalExpenses)
	}
	fmt.Printf("12. Deletion verified: TotalExpenses after delete=%.2f\n", postDeleteSummary.TotalExpenses)

	fmt.Println("\n✅ ALL 12 ARCHITECTURE SYNCHRONIZATION TESTS PASSED SUCCESSFULLY!")
}
