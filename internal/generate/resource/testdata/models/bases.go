package models

import (
	"example.com/shop/internal/common"
	ub "github.com/uptrace/bun"
)

// Stamp imports bun under another name: still a Bun model.
type Stamp struct {
	ub.BaseModel `bun:"table:stamps,alias:stamps"`

	ID    int64  `bun:"id,pk,autoincrement" json:"id"`
	Label string `bun:"label,notnull" json:"label"`
}

// BaseModel is the project's own base, embedding bun's: its user_id lands on every model that
// embeds it.
type BaseModel struct {
	ub.BaseModel

	UserID int64 `bun:"user_id,notnull" json:"-"`
}

// Safe embeds the project's own BaseModel, so it is owned.
type Safe struct {
	BaseModel `bun:"table:safes,alias:safes"`

	ID int64 `bun:"id,pk,autoincrement" json:"id"`
}

// Vault embeds a struct named BaseModel from another package: not bun's, so its columns are
// unknown.
type Vault struct {
	common.BaseModel `bun:"table:vaults,alias:vaults"`

	ID   int64  `bun:"id,pk,autoincrement" json:"id"`
	Name string `bun:"name,notnull" json:"name"`
}
