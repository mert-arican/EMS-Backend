package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

type Unit struct {
	Name      string `json:"name"`
	ManagerID int    `json:"managerID"`
}

func (Unit) CreateTableIfNotExists(s *Server) {
	query := `CREATE TABLE IF NOT EXISTS unit (
		name VARCHAR(256) PRIMARY KEY,
		manager_id INT
	)`
	_, err := s.DB.Exec(query)

	if err != nil {
		log.Fatal(err)
	}

	insertQuery := `INSERT INTO unit (name, manager_id)
	            SELECT 'Executive Management', 0
	            WHERE NOT EXISTS (SELECT 1 FROM unit WHERE name = 'Executive Management')`

	_, err = s.DB.Exec(insertQuery)

	if err != nil {
		log.Fatal(err)
	}
}

func (s *Server) CreateUnit(w http.ResponseWriter, r *http.Request) {
	var unit Unit
	if err := json.NewDecoder(r.Body).Decode(&unit); err != nil {
		http.Error(w, "Invalid JSON in request body", http.StatusBadRequest)
		return
	}

	query := `
        INSERT INTO unit (name, manager_id)
        VALUES ($1, $2)
    `

	_, err := s.DB.Exec(query, unit.Name, unit.ManagerID)
	if err != nil {
		log.Println("Failed to insert unit:", err)
		http.Error(w, "Failed to create unit", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(unit); err != nil {
		log.Println("Error encoding unit JSON:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		w.WriteHeader(http.StatusCreated)
	}
}

func (s *Server) GetUnit(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	var unit Unit
	err := s.DB.QueryRow("SELECT name, manager_id FROM unit WHERE name = $1", name).Scan(&unit.Name, &unit.ManagerID)
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
	if err := json.NewEncoder(w).Encode(unit); err != nil {
		log.Println("Error encoding unit JSON:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func (s *Server) UpdateUnit(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	var unit Unit

	// Decode JSON body into unit
	if err := json.NewDecoder(r.Body).Decode(&unit); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Ensure Name is valid
	if unit.Name == "" {
		http.Error(w, "Missing or invalid Name", http.StatusBadRequest)
		return
	}

	// Check if unit exists before update
	var exists bool
	err := s.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM unit WHERE name = $1)", name).Scan(&exists)
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
		UPDATE unit 
		SET name = $1, manager_id = $2
		WHERE name = $3
	`
	_, err = s.DB.Exec(query, unit.Name, unit.ManagerID, name)
	if err != nil {
		log.Printf("DB update error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Respond with updated unit
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(unit); err != nil {
		log.Printf("JSON encoding error: %v", err)
	}
}

func (s *Server) DeleteUnit(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]

	// Perform the DELETE query
	result, err := s.DB.Exec("DELETE FROM unit WHERE name = $1", name)
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
		http.Error(w, "Unit not found", http.StatusNotFound)
		return
	}

	// Return a success message (204 No Content is common for successful DELETE)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ListUnits(w http.ResponseWriter, r *http.Request) {
	// Parse query params for filtering (e.g., ?name=foo&managerID=123)
	queryParams := r.URL.Query()
	var filters []string
	var args []any
	argPos := 1

	if name := queryParams.Get("name"); name != "" {
		filters = append(filters, "name = $"+strconv.Itoa(argPos))
		args = append(args, name)
		argPos++
	}
	if managerID := queryParams.Get("manager_id"); managerID != "" {
		filters = append(filters, "manager_id = $"+strconv.Itoa(argPos))
		args = append(args, managerID)
		argPos++
	}

	// Build the SQL query
	query := "SELECT name, manager_id FROM unit"
	if len(filters) > 0 {
		query += " WHERE " + strings.Join(filters, " AND ")
	}

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		log.Println("Error querying units:", err)
		http.Error(w, "Failed to query units from database", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var allUnits []Unit
	for rows.Next() {
		var unit Unit
		if err := rows.Scan(&unit.Name, &unit.ManagerID); err != nil {
			log.Println("Error scanning unit row:", err)
			http.Error(w, "Failed to scan unit data", http.StatusInternalServerError)
			return
		}
		allUnits = append(allUnits, unit)
	}

	if err = rows.Err(); err != nil {
		log.Println("Row iteration error:", err)
		http.Error(w, "Error iterating over unit rows", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(allUnits); err != nil {
		log.Println("JSON encoding error:", err)
	}
}
