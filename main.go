package main

import (
	"database/sql"
	"log"
	"main/server"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

func main() {
	// postgresConnectionKey := "postgres://mertarican:secret@localhost:5432/se_project?sslmode=disable"
	// db, err := sql.Open("postgres", postgresConnectionKey)
	dsn := os.Getenv("POSTGRES_URL")
	if dsn == "" {
		log.Fatal("POSTGRES_URL not set")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	server := &server.Server{DB: db}

	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}

	createTablesIfNotExist(server)

	r := mux.NewRouter()

	// /user
	r.HandleFunc("/users", server.ListUsers).Methods("GET")
	r.HandleFunc("/users", server.CreateUser).Methods("POST")
	r.HandleFunc("/users/{id:[0-9]+}", server.GetUser).Methods("GET")
	r.HandleFunc("/users/{id:[0-9]+}", server.UpdateUser).Methods("PUT")
	r.HandleFunc("/users/{id:[0-9]+}", server.DeleteUser).Methods("DELETE")

	// /unit
	r.HandleFunc("/units", server.ListUnits).Methods("GET")
	r.HandleFunc("/units", server.CreateUnit).Methods("POST")
	r.HandleFunc("/units/{name}", server.GetUnit).Methods("GET")
	r.HandleFunc("/units/{name}", server.UpdateUnit).Methods("PUT")
	r.HandleFunc("/units/{name}", server.DeleteUnit).Methods("DELETE")

	// /expense_category
	r.HandleFunc("/expense_categories", server.ListExpenseCategories).Methods("GET")
	r.HandleFunc("/expense_categories", server.CreateExpenseCategory).Methods("POST")
	r.HandleFunc("/expense_categories/{name}", server.GetExpenseCategory).Methods("GET")
	r.HandleFunc("/expense_categories/{name}", server.UpdateExpenseCategory).Methods("PUT")
	r.HandleFunc("/expense_categories/{name}", server.DeleteExpenseCategory).Methods("DELETE")

	// /expense_request
	r.HandleFunc("/expense_requests", server.ListExpenseRequests).Methods("GET")
	r.HandleFunc("/expense_requests", server.CreateExpenseRequest).Methods("POST")
	r.HandleFunc("/expense_requests/{id:[0-9]+}", server.GetExpenseRequest).Methods("GET")
	r.HandleFunc("/expense_requests/{id:[0-9]+}", server.UpdateExpenseRequest).Methods("PUT")
	r.HandleFunc("/expense_requests/{id:[0-9]+}", server.DeleteExpenseRequest).Methods("DELETE")

	// /expense_activity
	r.HandleFunc("/expense_activities", server.ListExpenseActivities).Methods("GET")
	r.HandleFunc("/expense_activities", server.CreateExpenseActivity).Methods("POST")
	r.HandleFunc("/expense_activities/{id:[0-9]+}", server.GetExpenseActivity).Methods("GET")
	r.HandleFunc("/expense_activities/{id:[0-9]+}", server.UpdateExpenseActivity).Methods("PUT")
	r.HandleFunc("/expense_activities/{id:[0-9]+}", server.DeleteExpenseActivity).Methods("DELETE")

	// /paid_expense
	r.HandleFunc("/paid_expenses", server.ListPaidExpenses).Methods("GET")
	r.HandleFunc("/paid_expenses", server.CreatePaidExpense).Methods("POST")
	r.HandleFunc("/paid_expenses/{id:[0-9]+}", server.GetPaidExpense).Methods("GET")
	r.HandleFunc("/paid_expenses/{id:[0-9]+}", server.UpdatePaidExpense).Methods("PUT")
	r.HandleFunc("/paid_expenses/{id:[0-9]+}", server.DeletePaidExpense).Methods("DELETE")

	// /budget
	r.HandleFunc("/budgets", server.ListBudgets).Methods("GET")
	r.HandleFunc("/budgets", server.CreateBudget).Methods("POST")
	r.HandleFunc("/budgets/{unit_id}/{category}/{year:[0-9]+}", server.GetBudget).Methods("GET")
	r.HandleFunc("/budgets/{unit_id}/{category}/{year:[0-9]+}", server.UpdateBudget).Methods("PUT")
	r.HandleFunc("/budgets/{unit_id}/{category}/{year:[0-9]+}", server.DeleteBudget).Methods("DELETE")

	// /announcement
	r.HandleFunc("/announcements", server.ListAnnouncements).Methods("GET")
	r.HandleFunc("/announcements", server.CreateAnnouncement).Methods("POST")
	r.HandleFunc("/announcements/{id:[0-9]+}", server.GetAnnouncement).Methods("GET")
	r.HandleFunc("/announcements/{id:[0-9]+}", server.UpdateAnnouncement).Methods("PUT")
	r.HandleFunc("/announcements/{id:[0-9]+}", server.DeleteAnnouncement).Methods("DELETE")

	// Business logic
	r.HandleFunc("/expense_requests/{id}/pay", server.PayExpense).Methods("POST")

	log.Println("Listening on http://localhost:8080")
	http.ListenAndServe("0.0.0.0:8080", r)
}

type TableCreator interface {
	CreateTableIfNotExists(*server.Server)
}

func createTablesIfNotExist(s *server.Server) {
	creators := []TableCreator{
		server.User{},
		server.Unit{},
		server.ExpenseCategory{},
		server.ExpenseRequest{},
		server.ExpenseActivity{},
		server.PaidExpense{},
		server.Budget{},
		server.Announcement{},
	}

	for _, c := range creators {
		c.CreateTableIfNotExists(s)
	}
}
