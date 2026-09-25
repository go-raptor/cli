package models

import "github.com/uptrace/bun"

type (
	Tags = []Tag
	Tag  struct {
		bun.BaseModel `bun:"table:tags,alias:tags"`

		ID    int64  `bun:"id,pk,autoincrement" json:"id"`
		Label string `bun:"label,notnull" json:"label"`
	}
)
