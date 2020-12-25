package modules

import "time"

// WalletOps ...
type WalletOps string

const (
	WalletStore  WalletOps = "store"
	WalletDeduct WalletOps = "deduct"
	WalletOther  WalletOps = "other"
	WalletNone   WalletOps = "none"
)

// Wallet ...
type Wallet struct {
	ID       int `gorm:"primaryKey"`
	UserID   int
	Amount   float64
	Currency string
	UpdateAt time.Time
}
