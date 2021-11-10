package mysql

import (
	"database/sql"
	"errors"
	"goinvest/internal/invest"
)

type Storage struct {
	db *sql.DB
}

func NewStorage(db *sql.DB) (invest.Storage, error) {
	if db == nil {
		return nil, errors.New("db handle provided to city storage is nil")
	}
	return &Storage{db: db}, nil
}
