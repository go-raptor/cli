package models

import "github.com/uptrace/bun"

type Division struct {
	bun.BaseModel `bun:"table:divisions,alias:divisions"`

	ID   int64  `bun:"id,pk,autoincrement" json:"id"`
	Name string `bun:"name,notnull,unique" json:"name"`
}
