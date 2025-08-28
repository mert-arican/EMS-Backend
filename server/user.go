package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

type UserRole string

const (
	Admin          UserRole = "Admin"
	FieldPersonnel UserRole = "Personnel"
	Manager        UserRole = "Manager"
	Accounter      UserRole = "Accountant"
)

type User struct {
	ID       int      `json:"id,omitempty"`
	Name     string   `json:"name"`
	UnitID   string   `json:"unitID"`
	RoleID   UserRole `json:"roleID"`
	Password string   `json:"password"`
}

func (User) CreateTableIfNotExists(s *Server) {
	query := `CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(256) NOT NULL,
			unit_id VARCHAR(256) NOT NULL,
			role_id VARCHAR(64) NOT NULL,
			password VARCHAR(256) NOT NULL
	)`

	_, err := s.DB.Exec(query)

	if err != nil {
		log.Fatal(err)
	}

	query = `INSERT INTO users (name, unit_id, role_id, password)
	SELECT 'admin', 'ExecutiveManagement', 'admin', 'password'
	WHERE NOT EXISTS (
		SELECT 1 FROM users WHERE name = 'admin' AND role_id = 'admin'
	)`

	_, err = s.DB.Exec(query)

	if err != nil {
		log.Fatal(err)
	}
}

func (s *Server) CreateUser(w http.ResponseWriter, r *http.Request) {
	// Decode the user data from the request body
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Prepare the SQL query with RETURNING to get the generated ID
	query := `
        INSERT INTO users (name, unit_id, role_id, password)
        VALUES ($1, $2, $3, $4)
        RETURNING id
    `

	// Execute the query and retrieve the generated ID
	// var id int
	err := s.DB.QueryRow(query, user.Name, user.UnitID, user.RoleID, user.Password).Scan(&user.ID)
	if err != nil {
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		log.Println("Insert error:", err)
		return
	}

	// Set the response header and return the created user ID
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

func (s *Server) GetUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	var user User
	err = s.DB.QueryRow("SELECT * FROM users WHERE id = $1", id).Scan(
		&user.ID,
		&user.Name,
		&user.UnitID,
		&user.RoleID,
		&user.Password,
	)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(user)
}

func (s *Server) UpdateUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var user User

	// Decode JSON body into user
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Ensure ID is valid
	if id == 0 {
		http.Error(w, "Missing or invalid ID", http.StatusBadRequest)
		return
	}

	// Check if user exists before update
	var exists bool
	err = s.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", id).Scan(&exists)
	if err != nil {
		log.Printf("DB error checking user existence: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	if !exists {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Prepare the SQL UPDATE statement
	query := `
		UPDATE users
		SET name = $1, unit_id = $2, role_id = $3, password = $4
		WHERE id = $5
	`
	_, err = s.DB.Exec(query, user.Name, user.UnitID, user.RoleID, user.Password, id)
	if err != nil {
		log.Printf("DB update error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	user.ID = id
	// Respond with updated user
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(user); err != nil {
		log.Printf("JSON encoding error: %v", err)
	}
}

func (s *Server) DeleteUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	// Perform the DELETE query
	result, err := s.DB.Exec("DELETE FROM users WHERE id = $1", id)
	if err != nil {
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		log.Println("Delete error:", err)
		return
	}

	// Check if any rows were affected (i.e., if the user exists)
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "Error checking affected rows", http.StatusInternalServerError)
		log.Println("Rows affected error:", err)
		return
	}

	// If no rows were affected, return 404 (User not found)
	if rowsAffected == 0 {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Return a success message (204 No Content is common for successful DELETE)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ListUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filters := []string{}
	args := []interface{}{}
	idx := 1

	// Optional query parameters
	if unitID := r.URL.Query().Get("unit_id"); unitID != "" {
		filters = append(filters, "unit_id = $"+strconv.Itoa(idx))
		args = append(args, unitID)
		idx++
	}
	if roleID := r.URL.Query().Get("role_id"); roleID != "" {
		filters = append(filters, "role_id = $"+strconv.Itoa(idx))
		args = append(args, roleID)
		idx++
	}
	if name := r.URL.Query().Get("name"); name != "" {
		filters = append(filters, "name ILIKE $"+strconv.Itoa(idx)) // Case-insensitive search
		args = append(args, "%"+name+"%")
		idx++
	}

	query := "SELECT id, name, unit_id, role_id, password FROM users"
	if len(filters) > 0 {
		query += " WHERE " + strings.Join(filters, " AND ")
	}

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		http.Error(w, "Database query failed", http.StatusInternalServerError)
		log.Println("ListUsers query error:", err)
		return
	}
	defer rows.Close()

	var allUsers []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.UnitID, &u.RoleID, &u.Password); err != nil {
			http.Error(w, "Failed to scan user", http.StatusInternalServerError)
			log.Println("Row scan error:", err)
			return
		}
		allUsers = append(allUsers, u)
	}

	if err := rows.Err(); err != nil {
		http.Error(w, "Row iteration error", http.StatusInternalServerError)
		log.Println("Row iteration error:", err)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(allUsers); err != nil {
		http.Error(w, "JSON encoding failed", http.StatusInternalServerError)
		log.Println("Encoding error:", err)
	}
}
