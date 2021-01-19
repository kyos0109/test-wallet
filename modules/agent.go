package modules

import "time"

// Agent ...
type Agent struct {
	ID              string `gorm:"primaryKey"`
	Key             string
	PrivateKey      []byte
	UpdateAt        time.Time
	Status          bool
	SingleWalletUrl string
}
