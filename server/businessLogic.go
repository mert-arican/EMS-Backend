package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

// /expenses/{expense_id}/pay
// func (s *Server) payExpense(w http.ResponseWriter, r *http.Request) {
// fetch expense request
// fetch budget
// spent = fetch all paid so far for unit-category-year and sum result
// budget - spent = rest
// budgetMax = budget + ratio*budget

// if spent < budget && spent + amount > budget && spent + amount < maxBudget
//
// if spent < budget && spent + amount > budget && spent + amount > maxBudget
//
// if spent > budget && spent + amount > budget && spent + amount < maxBudget
//
// if spent > budget && spent + amount > budget && spent + amount > maxBudget
//
// }

func (s *Server) PayExpense(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	// 1. Fetch the PaidExpense
	var paid PaidExpense
	err = s.DB.QueryRow(`
		SELECT id, expense_id, unit_id, category, amount, created_at
		FROM paid_expense
		WHERE id = $1
	`, id).Scan(
		&paid.ID,
		&paid.ExpenseID,
		&paid.UnitID,
		&paid.Category,
		&paid.Amount,
		&paid.CreatedAt,
	)
	if err != nil {
		http.Error(w, "Paid expense not found", http.StatusNotFound)
		log.Println("Query error:", err)
		return
	}

	// 2. Fetch the corresponding ExpenseRequest to get year
	var createdAt time.Time
	err = s.DB.QueryRow(`
		SELECT created_at
		FROM expense_request
		WHERE id = $1
	`, paid.ExpenseID).Scan(&createdAt)
	if err != nil {
		http.Error(w, "Related expense request not found", http.StatusInternalServerError)
		log.Println("ExpenseRequest fetch error:", err)
		return
	}
	year := createdAt.Year()

	// 3. Fetch the Budget
	var budget Budget
	err = s.DB.QueryRow(`
		SELECT unit_id, expense_category AS category, year, budget_limit, threshold_ratio
		FROM budget
		WHERE unit_id = $1 AND expense_category = $2 AND year = $3
	`, paid.UnitID, paid.Category, year).Scan(
		&budget.UnitID,
		&budget.Category,
		&budget.Year,
		&budget.BudgetLimit,
		&budget.ThresholdRatio,
	)
	if err != nil {
		http.Error(w, "Budget not found", http.StatusInternalServerError)
		log.Println("Budget fetch error:", err)
		return
	}

	// 4. Sum all paid amounts for same unit-category-year
	var spent float64
	err = s.DB.QueryRow(`
		SELECT COALESCE(SUM(amount), 0)
		FROM paid_expense
		WHERE unit_id = $1 AND category = $2 AND EXTRACT(YEAR FROM created_at) = $3
	`, paid.UnitID, paid.Category, year).Scan(&spent)
	if err != nil {
		http.Error(w, "Failed to calculate spent amount", http.StatusInternalServerError)
		log.Println("Spent calculation error:", err)
		return
	}

	// 5. Compute rest and budgetMax
	rest := budget.BudgetLimit - spent
	budgetMax := budget.BudgetLimit + (budget.ThresholdRatio * budget.BudgetLimit)

	// 6. Send response
	resp := map[string]interface{}{
		"paidExpense": paid,
		"budget": map[string]interface{}{
			"year":      budget.Year,
			"limit":     budget.BudgetLimit,
			"threshold": budget.ThresholdRatio,
			"spent":     spent,
			"rest":      rest,
			"budgetMax": budgetMax,
		},
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(resp)
}

func sendAnnouncement(senderID int, receiverID int, message string) {
	// add entry to announcements
}
