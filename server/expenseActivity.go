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

type ExpenseState string

const (
	Pending         ExpenseState = "Pending"
	Approved        ExpenseState = "Approved"
	Rejected        ExpenseState = "Rejected"
	CategoryChanged ExpenseState = "CategoryChanged"
	Payed           ExpenseState = "Payed"
	PartiallyPayed  ExpenseState = "PartiallyPayed"
)

type ExpenseActivity struct {
	ID           int          `json:"id,omitempty"`
	ExpenseID    int          `json:"expenseID"`
	CurrentState ExpenseState `json:"currentState"`
	Feedback     string       `json:"feedback"`
	CreatedBy    int          `json:"createdBy"`
	CreatedAt    *time.Time   `json:"createdAt,omitempty"`
}

func (ExpenseActivity) CreateTableIfNotExists(s *Server) {
	query := `CREATE TABLE IF NOT EXISTS expense_activity (
		id SERIAL PRIMARY KEY,
		expense_id INT NOT NULL,
		current_state VARCHAR(256) NOT NULL,
		feedback TEXT NOT NULL,
		created_by INT NOT NULL,
		created_at timestamp DEFAULT NOW()
	)`

	_, err := s.DB.Exec(query)

	if err != nil {
		log.Fatal(err)
	}
}

func (s *Server) CreateExpenseActivity(w http.ResponseWriter, r *http.Request) {
	var expenseActivity ExpenseActivity
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	// Decode JSON body
	if err := decoder.Decode(&expenseActivity); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// Prepare SQL query
	query := `
		INSERT INTO expense_activity (expense_id, current_state, feedback, created_by)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`

	// Execute query and scan the result
	err := s.DB.QueryRow(query,
		expenseActivity.ExpenseID,
		expenseActivity.CurrentState,
		expenseActivity.Feedback,
		expenseActivity.CreatedBy,
	).Scan(&expenseActivity.ID, &expenseActivity.CreatedAt)

	if err != nil {
		log.Println("createExpenseActivity insert failed", err)
		http.Error(w, "Could not create expense activity", http.StatusInternalServerError)
		return
	}

	// Set response fields
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(expenseActivity); err != nil {
		log.Println("createExpenseActivity response encoding failed", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (s *Server) GetExpenseActivity(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	var expenseActivity ExpenseActivity
	err = s.DB.QueryRow(`
		SELECT id, expense_id, current_state, feedback, created_by, created_at
		FROM expense_activity
		WHERE id = $1
	`, id).Scan(
		&expenseActivity.ID,
		&expenseActivity.ExpenseID,
		&expenseActivity.CurrentState,
		&expenseActivity.Feedback,
		&expenseActivity.CreatedBy,
		&expenseActivity.CreatedAt,
	)

	if err != nil {
		// if errors.Is(err, sql.ErrNoRows) {
		// 	http.Error(w, "Expense activity not found", http.StatusNotFound)
		// 	return
		// }
		log.Println("getExpenseActivity query error:", err)
		http.Error(w, "Failed to retrieve expense activity", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(expenseActivity); err != nil {
		log.Println("getExpenseActivity response encoding error:", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (s *Server) UpdateExpenseActivity(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)

	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	var expenseActivity ExpenseActivity
	if err := json.NewDecoder(r.Body).Decode(&expenseActivity); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Prepare the SQL UPDATE statement
	query := `
		UPDATE expense_activity 
		SET expense_id = $1, current_state = $2, feedback = $3, created_by = $4
		WHERE id = $5
	`
	_, err = s.DB.Exec(
		query,
		expenseActivity.ExpenseID,
		expenseActivity.CurrentState,
		expenseActivity.Feedback,
		expenseActivity.CreatedBy,
		id,
	)

	if err != nil {
		log.Println("updateExpenseActivity update error:", err)
		http.Error(w, "Failed to update expense activity", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(expenseActivity); err != nil {
		log.Println("updateExpenseActivity response encoding error:", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (s *Server) DeleteExpenseActivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	result, err := s.DB.Exec("DELETE FROM expense_activity WHERE id = $1", id)
	if err != nil {
		log.Println("deleteExpenseActivity query error:", err)
		http.Error(w, "Failed to delete expense activity", http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Println("deleteExpenseActivity rowsAffected error:", err)
		http.Error(w, "Failed to determine result of deletion", http.StatusInternalServerError)
		return
	}
	if rowsAffected == 0 {
		http.Error(w, "Expense activity not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent) // 204 No Content
}

func (s *Server) ListExpenseActivities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filters := []string{}
	args := []interface{}{}
	idx := 1

	// Query param filters
	if expenseID := r.URL.Query().Get("expense_id"); expenseID != "" {
		filters = append(filters, "expense_id = $"+strconv.Itoa(idx))
		args = append(args, expenseID)
		idx++
	}
	if createdBy := r.URL.Query().Get("created_by"); createdBy != "" {
		filters = append(filters, "created_by = $"+strconv.Itoa(idx))
		args = append(args, createdBy)
		idx++
	}
	if state := r.URL.Query().Get("current_state"); state != "" {
		filters = append(filters, "current_state = $"+strconv.Itoa(idx))
		args = append(args, state)
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

	// Build SQL query
	query := `
		SELECT id, expense_id, current_state, feedback, created_by, created_at
		FROM expense_activity
	`
	if len(filters) > 0 {
		query += " WHERE " + strings.Join(filters, " AND ")
	}
	query += " ORDER BY created_at DESC"

	// Execute query
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		http.Error(w, "Database query failed", http.StatusInternalServerError)
		log.Println("ListExpenseActivities query error:", err)
		return
	}
	defer rows.Close()

	// Collect results
	var allActivities []ExpenseActivity
	for rows.Next() {
		var ea ExpenseActivity
		err := rows.Scan(&ea.ID, &ea.ExpenseID, &ea.CurrentState, &ea.Feedback, &ea.CreatedBy, &ea.CreatedAt)
		if err != nil {
			http.Error(w, "Failed to scan expense activity", http.StatusInternalServerError)
			log.Println("Row scan error:", err)
			return
		}
		allActivities = append(allActivities, ea)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "Row iteration error", http.StatusInternalServerError)
		log.Println("Row iteration error:", err)
		return
	}

	// Send JSON response
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(allActivities); err != nil {
		http.Error(w, "JSON encoding failed", http.StatusInternalServerError)
		log.Println("Encoding error:", err)
	}
}
