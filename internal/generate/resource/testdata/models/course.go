package models

import (
	"time"

	"github.com/uptrace/bun"
)

type Course struct {
	bun.BaseModel `bun:"table:courses,alias:courses"`

	ID         int64     `bun:"id,pk,autoincrement" json:"id"`
	UserID     int64     `bun:"user_id,notnull" json:"-"`
	DivisionID int64     `bun:"division_id,notnull" json:"divisionId"`
	Name       string    `bun:"name,notnull" json:"name"`
	CreatedAt  time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"createdAt"`
	UpdatedAt  time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updatedAt"`

	User     *User     `bun:"rel:belongs-to,join:user_id=id" json:"-"`
	Division *Division `bun:"rel:belongs-to,join:division_id=id" json:"division,omitempty"`
}
