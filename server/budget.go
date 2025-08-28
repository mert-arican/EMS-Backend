package server

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

type Budget struct {
	UnitID         string  `json:"unitID"`
	Category       string  `json:"category"`
	Year           int     `json:"year"`
	BudgetLimit    float64 `json:"budgetLimit"`
	ThresholdRatio float64 `json:"thresholdRatio"`
}

func (Budget) CreateTableIfNotExists(s *Server) {
	query := `CREATE TABLE IF NOT EXISTS budget (
		unit_id VARCHAR(256) NOT NULL,
		expense_category VARCHAR(256) NOT NULL,
		year INT NOT NULL,
		budget_limit NUMERIC NOT NULL,
		threshold_ratio NUMERIC NOT NULL,

		PRIMARY KEY (unit_id, expense_category, year)
	)`

	_, err := s.DB.Exec(query)

	if err != nil {
		log.Fatal(err)
	}
}

func (s *Server) CreateBudget(w http.ResponseWriter, r *http.Request) {
	// Decode JSON request body into Budget struct
	var budget Budget
	if err := json.NewDecoder(r.Body).Decode(&budget); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Insert into database
	query := `
		INSERT INTO budget (unit_id, expense_category, year, budget_limit, threshold_ratio)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := s.DB.Exec(
		query,
		budget.UnitID,
		budget.Category,
		budget.Year,
		budget.BudgetLimit,
		budget.ThresholdRatio,
	)
	if err != nil {
		log.Println("Insert budget error:", err)
		http.Error(w, "Failed to create budget", http.StatusInternalServerError)
		return
	}

	// Respond with 201 Created
	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(budget)
}

func (s *Server) GetBudget(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	vars := mux.Vars(r)
	unitID := vars["unit_id"]
	category := vars["category"]
	yearStr := vars["year"]
	// unitID := r.URL.Query().Get("unit_id")
	// category := r.URL.Query().Get("category")
	// yearStr := r.URL.Query().Get("year")

	if unitID == "" || category == "" || yearStr == "" {
		http.Error(w, "Missing required query parameters", http.StatusBadRequest)
		return
	}

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		http.Error(w, "Invalid year", http.StatusBadRequest)
		return
	}

	var budget Budget
	query := `
		SELECT unit_id, expense_category, year, budget_limit, threshold_ratio
		FROM budget
		WHERE unit_id = $1 AND expense_category = $2 AND year = $3
	`
	err = s.DB.QueryRow(query, unitID, category, year).Scan(
		&budget.UnitID,
		&budget.Category,
		&budget.Year,
		&budget.BudgetLimit,
		&budget.ThresholdRatio,
	)

	if err == sql.ErrNoRows {
		http.Error(w, "Budget not found", http.StatusNotFound)
		return
	} else if err != nil {
		log.Println("Get budget error:", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(budget)
}

func (s *Server) UpdateBudget(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	unitID := vars["unit_id"]
	category := vars["category"]
	yearStr := vars["year"]
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		http.Error(w, "Invalid year", http.StatusBadRequest)
		return
	}
	// Decode the JSON body
	var budget Budget
	if err := json.NewDecoder(r.Body).Decode(&budget); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Ensure all required fields are present
	if unitID == "" || category == "" || year == 0 {
		http.Error(w, "Missing required fields: unitID, category, or year", http.StatusBadRequest)
		return
	}

	// Check if budget record exists
	var exists bool
	checkQuery := `
		SELECT EXISTS (
			SELECT 1 FROM budget
			WHERE unit_id = $1 AND expense_category = $2 AND year = $3
		)
	`
	err = s.DB.QueryRow(checkQuery, unitID, category, year).Scan(&exists)
	if err != nil {
		log.Println("Error checking existence:", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	if !exists {
		http.Error(w, "Budget record not found", http.StatusNotFound)
		return
	}

	// Perform the update
	updateQuery := `
		UPDATE budget
		SET unit_id = $1, expense_category = $2, year = $3, budget_limit = $4, threshold_ratio = $5
		WHERE unit_id = $6 AND expense_category = $7 AND year = $8
	`
	_, err = s.DB.Exec(updateQuery,
		budget.UnitID,
		budget.Category,
		budget.Year,
		budget.BudgetLimit,
		budget.ThresholdRatio,
		unitID,
		category,
		year,
	)
	if err != nil {
		log.Println("Update error:", err)
		http.Error(w, "Failed to update budget", http.StatusInternalServerError)
		return
	}

	// Respond with updated budget
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(budget)
}

func (s *Server) DeleteBudget(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	// unitID := r.URL.Query().Get("unit_id")
	// category := r.URL.Query().Get("category")
	// yearStr := r.URL.Query().Get("year")
	vars := mux.Vars(r)
	unitID := vars["unit_id"]
	category := vars["category"]
	yearStr := vars["year"]

	if unitID == "" || category == "" || yearStr == "" {
		http.Error(w, "Missing required query parameters: unit_id, category, or year", http.StatusBadRequest)
		return
	}

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		http.Error(w, "Invalid year", http.StatusBadRequest)
		return
	}

	// Execute the DELETE query
	result, err := s.DB.Exec(`
		DELETE FROM budget
		WHERE unit_id = $1 AND expense_category = $2 AND year = $3
	`, unitID, category, year)
	if err != nil {
		log.Println("Delete error:", err)
		http.Error(w, "Failed to delete budget", http.StatusInternalServerError)
		return
	}

	// Check if any rows were affected
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "Failed to determine deletion result", http.StatusInternalServerError)
		return
	}
	if rowsAffected == 0 {
		http.Error(w, "Budget record not found", http.StatusNotFound)
		return
	}

	// Return 204 No Content
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ListBudgets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Build dynamic filters
	filters := []string{}
	args := []any{}
	idx := 1

	if unitID := r.URL.Query().Get("unit_id"); unitID != "" {
		filters = append(filters, "unit_id = $"+strconv.Itoa(idx))
		args = append(args, unitID)
		idx++
	}
	if category := r.URL.Query().Get("category"); category != "" {
		filters = append(filters, "expense_category = $"+strconv.Itoa(idx))
		args = append(args, category)
		idx++
	}
	if yearStr := r.URL.Query().Get("year"); yearStr != "" {
		if year, err := strconv.Atoi(yearStr); err == nil {
			filters = append(filters, "year = $"+strconv.Itoa(idx))
			args = append(args, year)
			idx++
		} else {
			http.Error(w, "Invalid year", http.StatusBadRequest)
			return
		}
	}

	// Construct query
	query := `SELECT unit_id, expense_category, year, budget_limit, threshold_ratio FROM budget`
	if len(filters) > 0 {
		query += " WHERE " + strings.Join(filters, " AND ")
	}

	// Execute query
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		log.Println("ListBudgets query error:", err)
		http.Error(w, "Database query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Parse results
	var budgets []Budget
	for rows.Next() {
		var b Budget
		err := rows.Scan(&b.UnitID, &b.Category, &b.Year, &b.BudgetLimit, &b.ThresholdRatio)
		if err != nil {
			log.Println("Row scan error:", err)
			http.Error(w, "Failed to read data", http.StatusInternalServerError)
			return
		}
		budgets = append(budgets, b)
	}

	if err := rows.Err(); err != nil {
		log.Println("Row iteration error:", err)
		http.Error(w, "Error reading results", http.StatusInternalServerError)
		return
	}

	// Return results as JSON
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(budgets); err != nil {
		log.Println("JSON encoding error:", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
