package models

import "github.com/uptrace/bun"

type LectureGroup struct {
	bun.BaseModel `bun:"table:lecture_groups,alias:lecture_groups"`

	ID     int64  `bun:"id,pk,autoincrement" json:"id"`
	UserID int64  `bun:"user_id,notnull" json:"-"`
	Name   string `bun:"name,notnull" json:"name"`
}
