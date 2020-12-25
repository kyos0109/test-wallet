package modules

import (
	"time"

	guuid "github.com/google/uuid"
)

// OrderStatus ...
type OrderStatus string

const (
	OrderOk     OrderStatus = "ok"
	OrderFailed OrderStatus = "failed"
	OrderOther  OrderStatus = "other"
	OrderCreate OrderStatus = "create"
)

// Order ...
type Order struct {
	ID           guuid.UUID `gorm:"primary_key;type:uuid;default:uuid_generate_v4()"`
	UserID       int
	WalletID     int
	GameID       int
	OpType       WalletOps
	RequestID    string
	BeforeAmount float64
	AfterAmount  float64
	Status       OrderStatus
	Comment      string
	CreateAt     time.Time
	UpdateAt     time.Time
}
