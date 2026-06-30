package handlers

import (
	"budgetapp/models"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

var jwtSecret = []byte("budget-app-secret-2024")

type Handler struct {
	db *sql.DB
}

func NewHandler(db *sql.DB) *Handler {
	return &Handler{db: db}
}

// hashPassword creates a salted SHA-256 hash
func hashPassword(password string) string {
	salt := make([]byte, 16)
	rand.Read(salt)
	saltHex := hex.EncodeToString(salt)
	hash := sha256.Sum256([]byte(saltHex + password))
	return saltHex + ":" + hex.EncodeToString(hash[:])
}

func checkPassword(password, stored string) bool {
	parts := strings.SplitN(stored, ":", 2)
	if len(parts) != 2 {
		return false
	}
	hash := sha256.Sum256([]byte(parts[0] + password))
	return hex.EncodeToString(hash[:]) == parts[1]
}

// ── Auth ──────────────────────────────────────────────────────────────────────

func (h *Handler) Register(c echo.Context) error {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request"})
	}
	if req.Email == "" || req.Password == "" || req.Name == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "all fields are required"})
	}

	hash := hashPassword(req.Password)
	res, err := h.db.Exec(`INSERT INTO users (email, password, name) VALUES (?, ?, ?)`,
		req.Email, hash, req.Name)
	if err != nil {
		return c.JSON(http.StatusConflict, echo.Map{"error": "email already exists"})
	}

	id, _ := res.LastInsertId()
	token, _ := generateToken(int(id))
	return c.JSON(http.StatusCreated, echo.Map{"token": token, "user": echo.Map{"id": id, "email": req.Email, "name": req.Name}})
}

func (h *Handler) Login(c echo.Context) error {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request"})
	}

	var user models.User
	err := h.db.QueryRow(`SELECT id, email, password, name FROM users WHERE email = ?`, req.Email).
		Scan(&user.ID, &user.Email, &user.Password, &user.Name)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid credentials"})
	}

	if !checkPassword(req.Password, user.Password) {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid credentials"})
	}

	token, _ := generateToken(user.ID)
	return c.JSON(http.StatusOK, echo.Map{"token": token, "user": echo.Map{"id": user.ID, "email": user.Email, "name": user.Name}})
}

func (h *Handler) GetMe(c echo.Context) error {
	uid := userID(c)
	var user models.User
	h.db.QueryRow(`SELECT id, email, name, created_at FROM users WHERE id = ?`, uid).
		Scan(&user.ID, &user.Email, &user.Name, &user.CreatedAt)
	return c.JSON(http.StatusOK, user)
}

// ── JWT Middleware ────────────────────────────────────────────────────────────

func (h *Handler) JWTMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		auth := c.Request().Header.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			return c.JSON(http.StatusUnauthorized, echo.Map{"error": "missing token"})
		}
		tokenStr := strings.TrimPrefix(auth, "Bearer ")
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})
		if err != nil || !token.Valid {
			return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid token"})
		}
		claims := token.Claims.(jwt.MapClaims)
		c.Set("user_id", int(claims["user_id"].(float64)))
		return next(c)
	}
}

// ── Budgets ───────────────────────────────────────────────────────────────────

func (h *Handler) GetBudgets(c echo.Context) error {
	uid := userID(c)
	rows, err := h.db.Query(`
		SELECT b.id, b.user_id, b.name, b.amount, b.category, b.color, b.created_at,
			COALESCE(SUM(CASE WHEN t.type='expense' THEN t.amount ELSE 0 END), 0) as spent
		FROM budgets b
		LEFT JOIN transactions t ON t.budget_id = b.id
		WHERE b.user_id = ?
		GROUP BY b.id
		ORDER BY b.created_at DESC`, uid)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	defer rows.Close()

	budgets := []models.Budget{}
	for rows.Next() {
		var b models.Budget
		rows.Scan(&b.ID, &b.UserID, &b.Name, &b.Amount, &b.Category, &b.Color, &b.CreatedAt, &b.Spent)
		budgets = append(budgets, b)
	}
	return c.JSON(http.StatusOK, budgets)
}

func (h *Handler) CreateBudget(c echo.Context) error {
	uid := userID(c)
	var b models.Budget
	if err := c.Bind(&b); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request"})
	}
	if b.Color == "" {
		b.Color = "#6366f1"
	}
	res, err := h.db.Exec(`INSERT INTO budgets (user_id, name, amount, category, color) VALUES (?, ?, ?, ?, ?)`,
		uid, b.Name, b.Amount, b.Category, b.Color)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	id, _ := res.LastInsertId()
	b.ID = int(id)
	b.UserID = uid
	return c.JSON(http.StatusCreated, b)
}

func (h *Handler) UpdateBudget(c echo.Context) error {
	uid := userID(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var b models.Budget
	c.Bind(&b)
	_, err := h.db.Exec(`UPDATE budgets SET name=?, amount=?, category=?, color=? WHERE id=? AND user_id=?`,
		b.Name, b.Amount, b.Category, b.Color, id, uid)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	b.ID = id
	return c.JSON(http.StatusOK, b)
}

func (h *Handler) DeleteBudget(c echo.Context) error {
	uid := userID(c)
	id, _ := strconv.Atoi(c.Param("id"))
	h.db.Exec(`DELETE FROM budgets WHERE id=? AND user_id=?`, id, uid)
	return c.JSON(http.StatusOK, echo.Map{"message": "deleted"})
}

// ── Transactions ──────────────────────────────────────────────────────────────

func (h *Handler) GetTransactions(c echo.Context) error {
	uid := userID(c)
	rows, err := h.db.Query(`
		SELECT id, user_id, budget_id, title, amount, type, category, date, note, created_at
		FROM transactions WHERE user_id = ? ORDER BY date DESC, created_at DESC LIMIT 100`, uid)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	defer rows.Close()

	txns := []models.Transaction{}
	for rows.Next() {
		var t models.Transaction
		rows.Scan(&t.ID, &t.UserID, &t.BudgetID, &t.Title, &t.Amount, &t.Type, &t.Category, &t.Date, &t.Note, &t.CreatedAt)
		txns = append(txns, t)
	}
	return c.JSON(http.StatusOK, txns)
}

func (h *Handler) CreateTransaction(c echo.Context) error {
	uid := userID(c)
	var t models.Transaction
	if err := c.Bind(&t); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid request"})
	}
	if t.Date == "" {
		t.Date = time.Now().Format("2006-01-02")
	}
	res, err := h.db.Exec(`INSERT INTO transactions (user_id, budget_id, title, amount, type, category, date, note) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		uid, t.BudgetID, t.Title, t.Amount, t.Type, t.Category, t.Date, t.Note)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	id, _ := res.LastInsertId()
	t.ID = int(id)
	t.UserID = uid
	return c.JSON(http.StatusCreated, t)
}

func (h *Handler) UpdateTransaction(c echo.Context) error {
	uid := userID(c)
	id, _ := strconv.Atoi(c.Param("id"))
	var t models.Transaction
	c.Bind(&t)
	_, err := h.db.Exec(`UPDATE transactions SET title=?, amount=?, type=?, category=?, date=?, note=?, budget_id=? WHERE id=? AND user_id=?`,
		t.Title, t.Amount, t.Type, t.Category, t.Date, t.Note, t.BudgetID, id, uid)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	t.ID = id
	return c.JSON(http.StatusOK, t)
}

func (h *Handler) DeleteTransaction(c echo.Context) error {
	uid := userID(c)
	id, _ := strconv.Atoi(c.Param("id"))
	h.db.Exec(`DELETE FROM transactions WHERE id=? AND user_id=?`, id, uid)
	return c.JSON(http.StatusOK, echo.Map{"message": "deleted"})
}

// ── Summary ───────────────────────────────────────────────────────────────────

func (h *Handler) GetSummary(c echo.Context) error {
	uid := userID(c)
	var summary models.Summary

	h.db.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM transactions WHERE user_id=? AND type='income'`, uid).Scan(&summary.TotalIncome)
	h.db.QueryRow(`SELECT COALESCE(SUM(amount),0) FROM transactions WHERE user_id=? AND type='expense'`, uid).Scan(&summary.TotalExpenses)
	summary.Balance = summary.TotalIncome - summary.TotalExpenses

	rows, _ := h.db.Query(`
		SELECT b.name, b.amount, COALESCE(SUM(CASE WHEN t.type='expense' THEN t.amount ELSE 0 END),0), b.color, b.category
		FROM budgets b LEFT JOIN transactions t ON t.budget_id = b.id
		WHERE b.user_id = ? GROUP BY b.id`, uid)
	defer rows.Close()
	for rows.Next() {
		var s models.BudgetStat
		rows.Scan(&s.Name, &s.Amount, &s.Spent, &s.Color, &s.Category)
		summary.BudgetStats = append(summary.BudgetStats, s)
	}

	monthRows, _ := h.db.Query(`
		SELECT strftime('%Y-%m', date) as month,
			COALESCE(SUM(CASE WHEN type='income' THEN amount ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN type='expense' THEN amount ELSE 0 END),0)
		FROM transactions WHERE user_id=?
		GROUP BY month ORDER BY month DESC LIMIT 6`, uid)
	defer monthRows.Close()
	for monthRows.Next() {
		var m models.MonthlyDataPoint
		monthRows.Scan(&m.Month, &m.Income, &m.Expense)
		summary.MonthlyData = append(summary.MonthlyData, m)
	}

	return c.JSON(http.StatusOK, summary)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func userID(c echo.Context) int {
	return c.Get("user_id").(int)
}

func generateToken(uid int) (string, error) {
	claims := jwt.MapClaims{
		"user_id": uid,
		"exp":     time.Now().Add(7 * 24 * time.Hour).Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(jwtSecret)
}

// suppress unused import warning
var _ = fmt.Sprintf
