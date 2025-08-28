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

type PaidExpense struct {
	ID        int        `json:"id"`
	ExpenseID int        `json:"expenseID"`
	UnitID    string     `json:"unitID"`
	Category  string     `json:"category"`
	Amount    float64    `json:"amount"`
	CreatedAt *time.Time `json:"createdAt,omitempty"`
}

func (PaidExpense) CreateTableIfNotExists(s *Server) {
	query := `CREATE TABLE IF NOT EXISTS paid_expense (
		id SERIAL PRIMARY KEY,
		expense_id INT NOT NULL,
		unit_id VARCHAR(256) NOT NULL,
		category VARCHAR(256) NOT NULL,
		amount NUMERIC(7,2) NOT NULL,
		created_at timestamp DEFAULT NOW()
	)`

	_, err := s.DB.Exec(query)

	if err != nil {
		log.Fatal(err)
	}
}

func (s *Server) CreatePaidExpense(w http.ResponseWriter, r *http.Request) {
	// Decode the paid expense data from the request body
	var expense PaidExpense
	if err := json.NewDecoder(r.Body).Decode(&expense); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Prepare the SQL query with RETURNING to get the generated ID and created_at
	query := `
        INSERT INTO paid_expense (expense_id, unit_id, category, amount)
        VALUES ($1, $2, $3, $4)
        RETURNING id, created_at
    `

	// Execute the query and retrieve the generated ID and created_at
	err := s.DB.QueryRow(query, expense.ExpenseID, expense.UnitID, expense.Category, expense.Amount).Scan(&expense.ID, &expense.CreatedAt)
	if err != nil {
		http.Error(w, "Failed to create paid expense", http.StatusInternalServerError)
		log.Println("Insert error:", err)
		return
	}

	// Set the response header and return the created paid expense
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(expense)
}

func (s *Server) GetPaidExpense(w http.ResponseWriter, r *http.Request) {
	// Extract the ID from the URL path
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	// Query the database for the paid expense
	var expense PaidExpense
	err = s.DB.QueryRow("SELECT id, expense_id, unit_id, category, amount, created_at FROM paid_expense WHERE id = $1", id).Scan(
		&expense.ID,
		&expense.ExpenseID,
		&expense.UnitID,
		&expense.Category,
		&expense.Amount,
		&expense.CreatedAt,
	)
	if err != nil {
		http.Error(w, "Paid expense not found", http.StatusNotFound)
		log.Println("Query error:", err)
		return
	}

	// Respond with the JSON-encoded paid expense
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(expense)
}

func (s *Server) UpdatePaidExpense(w http.ResponseWriter, r *http.Request) {
	// Extract ID from URL
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	// Decode JSON body into PaidExpense struct
	var expense PaidExpense
	if err := json.NewDecoder(r.Body).Decode(&expense); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Ensure expense ID is set
	if expense.ID == 0 {
		http.Error(w, "Missing or invalid ID in body", http.StatusBadRequest)
		return
	}

	// Check if the paid expense exists
	var exists bool
	err = s.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM paid_expense WHERE id = $1)", id).Scan(&exists)
	if err != nil {
		log.Printf("DB error checking paid expense existence: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	if !exists {
		http.Error(w, "Paid expense not found", http.StatusNotFound)
		return
	}

	// Perform the update (we do not update created_at)
	query := `
		UPDATE paid_expense
		SET expense_id = $1, unit_id = $2, category = $3, amount = $4
		WHERE id = $5
	`
	_, err = s.DB.Exec(query, expense.ExpenseID, expense.UnitID, expense.Category, expense.Amount, id)
	if err != nil {
		log.Printf("DB update error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Respond with the updated paid expense
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(expense); err != nil {
		log.Printf("JSON encoding error: %v", err)
	}
}

func (s *Server) DeletePaidExpense(w http.ResponseWriter, r *http.Request) {
	// Extract ID from URL
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	// Perform the DELETE query
	result, err := s.DB.Exec("DELETE FROM paid_expense WHERE id = $1", id)
	if err != nil {
		http.Error(w, "Failed to delete paid expense", http.StatusInternalServerError)
		log.Println("Delete error:", err)
		return
	}

	// Check if any rows were affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "Error checking affected rows", http.StatusInternalServerError)
		log.Println("Rows affected error:", err)
		return
	}

	if rowsAffected == 0 {
		http.Error(w, "Paid expense not found", http.StatusNotFound)
		return
	}

	// Return 204 No Content on successful deletion
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ListPaidExpenses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filters := []string{}
	args := []any{}
	idx := 1

	// Optional query parameters
	if expenseID := r.URL.Query().Get("expense_id"); expenseID != "" {
		filters = append(filters, "expense_id = $"+strconv.Itoa(idx))
		args = append(args, expenseID)
		idx++
	}
	if unitID := r.URL.Query().Get("unit_id"); unitID != "" {
		filters = append(filters, "unit_id = $"+strconv.Itoa(idx))
		args = append(args, unitID)
		idx++
	}
	if category := r.URL.Query().Get("category"); category != "" {
		filters = append(filters, "category = $"+strconv.Itoa(idx))
		args = append(args, category)
		idx++
	}
	if minAmount := r.URL.Query().Get("min_amount"); minAmount != "" {
		filters = append(filters, "amount >= $"+strconv.Itoa(idx))
		args = append(args, minAmount)
		idx++
	}
	if maxAmount := r.URL.Query().Get("max_amount"); maxAmount != "" {
		filters = append(filters, "amount <= $"+strconv.Itoa(idx))
		args = append(args, maxAmount)
		idx++
	}
	if year := r.URL.Query().Get("year"); year != "" {
		filters = append(filters, "EXTRACT(YEAR FROM created_at) = $"+strconv.Itoa(idx))
		args = append(args, year)
		idx++
	}
	if month := r.URL.Query().Get("month"); month != "" {
		filters = append(filters, "EXTRACT(MONTH FROM created_at) = $"+strconv.Itoa(idx))
		args = append(args, month)
		idx++
	}
	if day := r.URL.Query().Get("day"); day != "" {
		filters = append(filters, "EXTRACT(DAY FROM created_at) = $"+strconv.Itoa(idx))
		args = append(args, day)
		idx++
	}

	query := "SELECT id, expense_id, unit_id, category, amount, created_at FROM paid_expense"
	if len(filters) > 0 {
		query += " WHERE " + strings.Join(filters, " AND ")
	}

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		http.Error(w, "Database query failed", http.StatusInternalServerError)
		log.Println("ListPaidExpenses query error:", err)
		return
	}
	defer rows.Close()

	var expenses []PaidExpense
	for rows.Next() {
		var pe PaidExpense
		if err := rows.Scan(&pe.ID, &pe.ExpenseID, &pe.UnitID, &pe.Category, &pe.Amount, &pe.CreatedAt); err != nil {
			http.Error(w, "Failed to scan paid expense", http.StatusInternalServerError)
			log.Println("Row scan error:", err)
			return
		}
		expenses = append(expenses, pe)
	}

	if err := rows.Err(); err != nil {
		http.Error(w, "Row iteration error", http.StatusInternalServerError)
		log.Println("Row iteration error:", err)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(expenses); err != nil {
		http.Error(w, "JSON encoding failed", http.StatusInternalServerError)
		log.Println("Encoding error:", err)
	}
}
