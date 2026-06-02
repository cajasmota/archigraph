package models

import (
	"time"

	"gorm.io/gorm"
)

// Account embeds gorm.Model -> soft_delete (deleted_at) + timestamps, plus an
// explicit created_by audit column.
type Account struct {
	gorm.Model
	Name      string `gorm:"column:name"`
	CreatedBy uint   `gorm:"column:created_by"`
	UpdatedBy uint   `gorm:"column:updated_by"`
}

// Invoice uses an explicit gorm.DeletedAt field (no gorm.Model embed) with a
// custom column, plus explicit CreatedAt+UpdatedAt timestamp columns.
type Invoice struct {
	ID         uint           `gorm:"primaryKey;column:id"`
	Amount     int            `gorm:"column:amount"`
	CreatedAt  time.Time      `gorm:"column:created_at"`
	UpdatedAt  time.Time      `gorm:"column:updated_at"`
	ArchivedAt gorm.DeletedAt `gorm:"column:archived_at"`
}

// Ledger is a plain GORM model with a `deleted` boolean but NO soft-delete
// library, DeletedAt type, or deleted_at column convention. It must NOT be
// classified as soft-delete (honesty boundary) and has no timestamp columns.
type Ledger struct {
	ID      uint   `gorm:"primaryKey;column:id"`
	Deleted bool   `gorm:"column:deleted"`
	Note    string `gorm:"column:note"`
}
