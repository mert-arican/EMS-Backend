package server

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

type Announcement struct {
	ID         int       `json:"id,omitempty"`
	Message    string    `json:"message"`
	ReceiverID int       `json:"receiverID"`
	CreatedBy  int       `json:"createdBy"`
	CreatedAt  time.Time `json:"createdAt"`
}

// type AAAnnouncement struct {
// 	ID         int        `json:"id,omitempty"`
// 	Message    string     `json:"message"`
// 	ReceiverID int        `json:"receiverID"`
// 	CreatedBy  int        `json:"createdBy"`
// 	CreatedAt  time.Time `json:"createdAt,omitempty"`
// }

func (Announcement) CreateTableIfNotExists(s *Server) {
	query := `CREATE TABLE IF NOT EXISTS announcement (
		id SERIAL PRIMARY KEY,
		message TEXT NOT NULL,
		receiver_id INT,
		created_by INT NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT NOW()
	)`

	_, err := s.DB.Exec(query)

	if err != nil {
		log.Fatal(err)
	}
}

func (s *Server) CreateAnnouncement(w http.ResponseWriter, r *http.Request) {
	var a Announcement

	// Decode request body into the Announcement struct
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		log.Printf("json:")
		return
	}

	// Validate required fields
	if a.Message == "" || a.CreatedBy == 0 {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		log.Printf("asda:")
		return
	}

	// Insert the announcement into the database
	query := `
		INSERT INTO announcement (message, receiver_id, created_by)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`
	err := s.DB.QueryRow(query, a.Message, a.ReceiverID, a.CreatedBy).
		Scan(&a.ID, &a.CreatedAt)
	if err != nil {
		log.Printf("CreateAnnouncement DB error: %v", err)
		http.Error(w, "Database insert failed", http.StatusInternalServerError)
		return
	}
	// Respond with the newly created announcement
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(a)
}

func (s *Server) GetAnnouncement(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var a Announcement
	query := `
		SELECT id, message, receiver_id, created_by, created_at
		FROM announcement
		WHERE id = $1
	`
	err = s.DB.QueryRow(query, id).Scan(
		&a.ID,
		&a.Message,
		&a.ReceiverID,
		&a.CreatedBy,
		&a.CreatedAt,
	)
	if err == sql.ErrNoRows {
		http.Error(w, "Announcement not found", http.StatusNotFound)
		return
	} else if err != nil {
		log.Printf("GetAnnouncement DB error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(a)
}

func (s *Server) UpdateAnnouncement(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var a Announcement
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	query := `
		UPDATE announcement
		SET message = $1, receiver_id = $2
		WHERE id = $3
	`
	result, err := s.DB.Exec(query, a.Message, a.ReceiverID, id)
	if err != nil {
		log.Printf("UpdateAnnouncement error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "Error checking affected rows", http.StatusInternalServerError)
		return
	}
	if rowsAffected == 0 {
		http.Error(w, "Announcement not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) DeleteAnnouncement(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	result, err := s.DB.Exec("DELETE FROM announcement WHERE id = $1", id)
	if err != nil {
		log.Printf("DeleteAnnouncement error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "Error checking affected rows", http.StatusInternalServerError)
		return
	}
	if rowsAffected == 0 {
		http.Error(w, "Announcement not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ListAnnouncements(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filters := []string{}
	args := []any{}
	idx := 1

	// Optional query parameters
	if receiverID := r.URL.Query().Get("receiver_id"); receiverID != "" {
		filters = append(filters, "receiver_id = $"+strconv.Itoa(idx))
		args = append(args, receiverID)
		idx++
	}
	if createdBy := r.URL.Query().Get("created_by"); createdBy != "" {
		filters = append(filters, "created_by = $"+strconv.Itoa(idx))
		args = append(args, createdBy)
		idx++
	}
	if message := r.URL.Query().Get("message"); message != "" {
		filters = append(filters, "message ILIKE $"+strconv.Itoa(idx))
		args = append(args, "%"+message+"%")
		idx++
	}

	query := "SELECT id, message, receiver_id, created_by, created_at FROM announcement"
	if len(filters) > 0 {
		query += " WHERE " + strings.Join(filters, " AND ")
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		http.Error(w, "Database query failed", http.StatusInternalServerError)
		log.Println("ListAnnouncements error:", err)
		return
	}
	defer rows.Close()

	var announcements []Announcement
	for rows.Next() {
		var a Announcement
		if err := rows.Scan(&a.ID, &a.Message, &a.ReceiverID, &a.CreatedBy, &a.CreatedAt); err != nil {
			http.Error(w, "Failed to scan announcement", http.StatusInternalServerError)
			log.Println("Scan error:", err)
			return
		}
		announcements = append(announcements, a)
	}

	if err := rows.Err(); err != nil {
		http.Error(w, "Row iteration error", http.StatusInternalServerError)
		log.Println("Iteration error:", err)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(announcements); err != nil {
		http.Error(w, "Encoding error", http.StatusInternalServerError)
		log.Println("Encoding error:", err)
	}
}
