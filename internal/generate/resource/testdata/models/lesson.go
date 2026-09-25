package models

import "github.com/uptrace/bun"

type Lesson struct {
	bun.BaseModel `bun:"table:lessons,alias:lessons"`

	ID         int64 `bun:"id,pk,autoincrement" json:"id"`
	OutcomeID  int64 `bun:"outcome_id,notnull" json:"outcomeId"`
	DivisionID int64 `bun:"division_id,notnull" json:"divisionId"`
}
