package models

import "github.com/uptrace/bun"

type UnitType string

type Unit struct {
	bun.BaseModel `bun:"table:units,alias:units"`

	ID        int64    `bun:"id,pk,autoincrement" json:"id"`
	OutcomeID int64    `bun:"outcome_id,notnull" json:"outcomeId"`
	Type      UnitType `bun:"type,notnull" json:"type"`
	Title     string   `bun:",notnull" json:"title"`
}
