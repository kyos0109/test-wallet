package modules

import "time"

// User ...
type User struct {
	ID       int `gorm:"primaryKey"`
	AgentID  int
	Name     string
	Status   bool
	UpdateAt time.Time
}
