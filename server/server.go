package server

import (
	"database/sql"
)

type Server struct {
	DB *sql.DB
}
