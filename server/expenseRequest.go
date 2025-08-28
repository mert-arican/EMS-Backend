package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

type ExpenseRequest struct {
	ID          int        `json:"id,omitempty"`
	UserID      int        `json:"userID"`
	UnitID      string     `json:"unitID"`
	Amount      float64    `json:"amount"`
	Category    string     `json:"category"`
	CreatedAt   *time.Time `json:"createdAt,omitempty"`
	IsFinalized bool       `json:"isFinalized"`
}

func (ExpenseRequest) CreateTableIfNotExists(s *Server) {
	query := `CREATE TABLE IF NOT EXISTS expense_request (
		id SERIAL PRIMARY KEY,
		user_id INT NOT NULL,
		unit_id VARCHAR(256) NOT NULL,
		amount NUMERIC(7,2) NOT NULL,
		category VARCHAR(256) NOT NULL,
		created_at timestamp DEFAULT NOW(),
		is_finalized BOOLEAN
	)`

	_, err := s.DB.Exec(query)

	if err != nil {
		log.Fatal(err)
	}
}

func (s *Server) CreateExpenseRequest(w http.ResponseWriter, r *http.Request) {
	var expenseRequest ExpenseRequest
	if err := json.NewDecoder(r.Body).Decode(&expenseRequest); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	query := `
		INSERT INTO expense_request (user_id, unit_id, amount, category, is_finalized)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`

	err := s.DB.QueryRow(query,
		expenseRequest.UserID,
		expenseRequest.UnitID,
		expenseRequest.Amount,
		expenseRequest.Category,
		expenseRequest.IsFinalized,
	).Scan(
		&expenseRequest.ID,
		&expenseRequest.CreatedAt,
	)
	if err != nil {
		log.Println("Insert error:", err)
		http.Error(w, "Failed to create expense", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(expenseRequest); err != nil {
		log.Println("JSON encode error:", err)
	}
}

func (s *Server) GetExpenseRequest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	var expenseRequest ExpenseRequest
	err = s.DB.QueryRow(`
		SELECT id, user_id, unit_id, amount, category, created_at, is_finalized
		FROM expense_request
		WHERE id = $1
	`, id).Scan(
		&expenseRequest.ID,
		&expenseRequest.UserID,
		&expenseRequest.UnitID,
		&expenseRequest.Amount,
		&expenseRequest.Category,
		&expenseRequest.CreatedAt,
		&expenseRequest.IsFinalized,
	)
	if err != nil {
		// if err == sql.ErrNoRows {
		// 	http.Error(w, "Expense request not found", http.StatusNotFound)
		// 	return
		// }
		log.Printf("Database error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(expenseRequest); err != nil {
		log.Printf("JSON encode error: %v", err)
	}
}

func (s *Server) UpdateExpenseRequest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var expenseRequest ExpenseRequest
	if err := json.NewDecoder(r.Body).Decode(&expenseRequest); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	query := `
		UPDATE expense_request
		SET user_id = $1, unit_id = $2, amount = $3, category = $4, is_finalized = $5
		WHERE id = $6
	`

	res, err := s.DB.Exec(query,
		expenseRequest.UserID,
		expenseRequest.UnitID,
		expenseRequest.Amount,
		expenseRequest.Category,
		expenseRequest.IsFinalized,
		id,
	)

	if err != nil {
		log.Printf("DB update error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		http.Error(w, "Error checking update result", http.StatusInternalServerError)
		return
	}
	if rowsAffected == 0 {
		http.Error(w, "Expense request not found", http.StatusNotFound)
		return
	}

	// // Set ID, but we can't get CreatedAt here because Exec doesn't return rows
	// expenseRequest.ID = id
	// Optionally: You can fetch CreatedAt separately if you want (optional step)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(expenseRequest); err != nil {
		log.Printf("JSON encode error: %v", err)
	}
}

func (s *Server) DeleteExpenseRequest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	result, err := s.DB.Exec("DELETE FROM expense_request WHERE id = $1", id)
	if err != nil {
		http.Error(w, "Failed to delete expense request", http.StatusInternalServerError)
		log.Printf("Delete error: %v", err)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "Error checking affected rows", http.StatusInternalServerError)
		log.Printf("Rows affected error: %v", err)
		return
	}

	if rowsAffected == 0 {
		http.Error(w, "Expense request not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent) // 204 No Content
}

func (s *Server) ListExpenseRequests(w http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()
	var filters []string
	var args []interface{}
	argPos := 1

	// if id := queryParams.Get("id"); id != "" {
	// 	filters = append(filters, "id = $"+strconv.Itoa(argPos))
	// 	idInt, err := strconv.Atoi(id)
	// 	if err != nil {
	// 		http.Error(w, "Invalid id parameter", http.StatusBadRequest)
	// 		return
	// 	}
	// 	args = append(args, idInt)
	// 	argPos++
	// }

	if userID := queryParams.Get("user_id"); userID != "" {
		filters = append(filters, "user_id = $"+strconv.Itoa(argPos))
		userIDInt, err := strconv.Atoi(userID)
		if err != nil {
			http.Error(w, "Invalid userID parameter", http.StatusBadRequest)
			return
		}
		args = append(args, userIDInt)
		argPos++
	}

	if unitID := queryParams.Get("unit_id"); unitID != "" {
		filters = append(filters, "unit_id = $"+strconv.Itoa(argPos))
		args = append(args, unitID)
		argPos++
	}

	if amount := queryParams.Get("amount"); amount != "" {
		filters = append(filters, "amount = $"+strconv.Itoa(argPos))
		amountFloat, err := strconv.ParseFloat(amount, 64)
		if err != nil {
			http.Error(w, "Invalid amount parameter", http.StatusBadRequest)
			return
		}
		args = append(args, amountFloat)
		argPos++
	}

	if category := queryParams.Get("category"); category != "" {
		filters = append(filters, "category = $"+strconv.Itoa(argPos))
		args = append(args, category)
		argPos++
	}

	if isFinalized := queryParams.Get("is_finalized"); isFinalized != "" {
		filters = append(filters, "is_finalized = $"+strconv.Itoa(argPos))
		isFinalizedBool, err := strconv.ParseBool(isFinalized)
		if err != nil {
			http.Error(w, "Invalid isFinalized parameter", http.StatusBadRequest)
			return
		}
		args = append(args, isFinalizedBool)
		argPos++
	}

	// Build the query string
	query := `
		SELECT id, user_id, unit_id, amount, category, created_at, is_finalized
		FROM expense_request
	`
	if len(filters) > 0 {
		query += " WHERE " + strings.Join(filters, " AND ")
	}

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		http.Error(w, "Failed to fetch expense requests", http.StatusInternalServerError)
		log.Printf("Query error: %v", err)
		return
	}
	defer rows.Close()

	expenses := []ExpenseRequest{}
	for rows.Next() {
		var expense ExpenseRequest
		err := rows.Scan(
			&expense.ID,
			&expense.UserID,
			&expense.UnitID,
			&expense.Amount,
			&expense.Category,
			&expense.CreatedAt,
			&expense.IsFinalized,
		)
		if err != nil {
			http.Error(w, "Failed to read expense request", http.StatusInternalServerError)
			log.Printf("Scan error: %v", err)
			return
		}
		expenses = append(expenses, expense)
	}

	if err := rows.Err(); err != nil {
		http.Error(w, "Error reading rows", http.StatusInternalServerError)
		log.Printf("Rows error: %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(expenses)
}
