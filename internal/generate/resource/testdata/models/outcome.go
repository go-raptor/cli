package models

import "github.com/uptrace/bun"

type Outcome struct {
	bun.BaseModel `bun:"table:outcomes,alias:outcomes"`

	ID       int64  `bun:"id,pk,autoincrement" json:"id"`
	CourseID int64  `bun:"course_id,notnull" json:"courseId"`
	Name     string `bun:"name,notnull" json:"name"`

	Course *Course `bun:"rel:belongs-to,join:course_id=id" json:"-"`
}
