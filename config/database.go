package config

import "time"

type db struct {
	SetMaxOpenConns    int
	SetMaxIdleConns    int
	SetConnMaxIdleTime time.Duration
}

var (
	DB = &db{
		SetMaxOpenConns:    200,
		SetMaxIdleConns:    50,
		SetConnMaxIdleTime: time.Hour,
	}
)
