package config

import "time"

type http struct {
	ClientTimeout time.Duration
}

var (
	Http = &http{
		ClientTimeout: 15 * time.Second,
	}
)
