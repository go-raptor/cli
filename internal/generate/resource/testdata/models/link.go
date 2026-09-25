package models

import "github.com/uptrace/bun"

type Link struct {
	bun.BaseModel `bun:"table:links,alias:links"`

	ID        int64 `bun:"id,pk,autoincrement" json:"id"`
	CourseID  int64 `bun:"course_id,notnull" json:"courseId"`
	OutcomeID int64 `bun:"outcome_id,notnull" json:"outcomeId"`
}
