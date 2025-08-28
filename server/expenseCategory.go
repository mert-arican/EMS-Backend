package server

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

type ExpenseCategory struct {
	Name string `json:"name"`
}

func (ExpenseCategory) CreateTableIfNotExists(s *Server) {
	query := `CREATE TABLE IF NOT EXISTS expense_category (
		name VARCHAR(256) PRIMARY KEY
	)`

	_, err := s.DB.Exec(query)

	if err != nil {
		log.Fatal(err)
	}
}

func (s *Server) CreateExpenseCategory(w http.ResponseWriter, r *http.Request) {
	var expenseCategory ExpenseCategory
	if err := json.NewDecoder(r.Body).Decode(&expenseCategory); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	query := `
		INSERT INTO expense_category (name)
		VALUES ($1)
	`

	_, err := s.DB.Exec(query,
		expenseCategory.Name,
	)
	if err != nil {
		log.Println("Insert error:", err)
		http.Error(w, "Failed to create expense", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(expenseCategory); err != nil {
		log.Println("JSON encode error:", err)
	}
}

func (s *Server) GetExpenseCategory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	var category ExpenseCategory
	err := s.DB.QueryRow("SELECT name FROM expense_category WHERE name = $1", name).Scan(&category.Name)
	if err != nil {
		// if err == sql.ErrNoRows {
		// 	http.Error(w, "Unit not found", http.StatusNotFound)
		// 	return
		// }
		// Log the error but do not exit
		log.Println("Error querying unit:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(category); err != nil {
		log.Println("Error encoding unit JSON:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (s *Server) UpdateExpenseCategory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	var category ExpenseCategory

	// Decode JSON body into unit
	if err := json.NewDecoder(r.Body).Decode(&category); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Ensure Name is valid
	if category.Name == "" {
		http.Error(w, "Missing or invalid Name", http.StatusBadRequest)
		return
	}

	// Check if unit exists before update
	var exists bool
	err := s.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM expense_category WHERE name = $1)", name).Scan(&exists)
	if err != nil {
		log.Printf("DB error checking unit existence: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	if !exists {
		http.Error(w, "Unit not found", http.StatusNotFound)
		return
	}

	// Prepare the SQL UPDATE statement
	query := `
		UPDATE expense_category 
		SET name = $1 WHERE name = $2
	`
	_, err = s.DB.Exec(query, category.Name, name)
	if err != nil {
		log.Printf("DB update error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Respond with updated unit
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(category); err != nil {
		log.Printf("JSON encoding error: %v", err)
	}
}

func (s *Server) DeleteExpenseCategory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	// Perform the DELETE query
	result, err := s.DB.Exec("DELETE FROM expense_category WHERE name = $1", name)
	if err != nil {
		http.Error(w, "Failed to delete unit", http.StatusInternalServerError)
		log.Println("Delete error:", err)
		return
	}

	// Check if any rows were affected (i.e., if the unit exists)
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "Error checking affected rows", http.StatusInternalServerError)
		log.Println("Rows affected error:", err)
		return
	}

	// If no rows were affected, return 404 (Unit not found)
	if rowsAffected == 0 {
		http.Error(w, "Category not found", http.StatusNotFound)
		return
	}

	// Return a success message (204 No Content is common for successful DELETE)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ListExpenseCategories(w http.ResponseWriter, r *http.Request) {
	// Build the SQL query
	query := "SELECT name FROM expense_category"

	rows, err := s.DB.Query(query)
	if err != nil {
		log.Println("Error querying categories:", err)
		http.Error(w, "Failed to query units from database", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var allCategories []ExpenseCategory
	for rows.Next() {
		var category ExpenseCategory
		if err := rows.Scan(&category.Name); err != nil {
			log.Println("Error scanning category row:", err)
			http.Error(w, "Failed to scan category data", http.StatusInternalServerError)
			return
		}
		allCategories = append(allCategories, category)
	}

	if err = rows.Err(); err != nil {
		log.Println("Row iteration error:", err)
		http.Error(w, "Error iterating over unit rows", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(allCategories); err != nil {
		log.Println("JSON encoding error:", err)
	}
}
