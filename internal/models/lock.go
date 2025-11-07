package models

// AppLock represents a distributed lock in the database
// Used for coordinating work between multiple instances in HA mode
type AppLock struct {
	LockName   string `gorm:"primaryKey;size:255"`
	InstanceID string `gorm:"size:255;not null"`
	AcquiredAt int64  `gorm:"not null;index"`
	ExpiresAt  int64  `gorm:"not null;index"`
}

// TableName specifies the table name for GORM
func (AppLock) TableName() string {
	return "app_locks"
}
