package modules

import (
	"time"

	guuid "github.com/google/uuid"
)

// APIRequestIDs ...
type APIRequestIDs struct {
	ID       guuid.UUID `gorm:"primary_key;type:uuid"`
	IP       string
	CreateAt time.Time
}
