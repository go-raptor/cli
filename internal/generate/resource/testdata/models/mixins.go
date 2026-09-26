package models

import (
	"time"

	"example.com/shop/internal/audit"
	"example.com/shop/internal/media"
	"github.com/uptrace/bun"
)

// Owned is a mixin: its user_id column lands on the table of the model that embeds it.
type Owned struct {
	UserID int64 `bun:"user_id,notnull" json:"-"`
	User   *User `bun:"rel:belongs-to,join:user_id=id" json:"-"`
}

type Stamps struct {
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"createdAt"`
}

// OwnedStamped nests both mixins, for recursive promotion.
type OwnedStamped struct {
	Owned
	Stamps
}

// Folder is owned through the embedded mixin.
type Folder struct {
	bun.BaseModel `bun:"table:folders,alias:folders"`

	ID int64 `bun:"id,pk,autoincrement" json:"id"`
	Owned
	Name string `bun:"name,notnull" json:"name"`
}

// Shelf embeds the nested mixin through a pointer.
type Shelf struct {
	bun.BaseModel `bun:"table:shelves,alias:shelves"`

	ID int64 `bun:"id,pk,autoincrement" json:"id"`
	*OwnedStamped
}

// Doc has no owner column of its own: it is owned through its folder.
type Doc struct {
	bun.BaseModel `bun:"table:docs,alias:docs"`

	ID       int64 `bun:"id,pk,autoincrement" json:"id"`
	FolderID int64 `bun:"folder_id,notnull" json:"folderId"`
}

// Album names its owner column owner_id.
type Album struct {
	bun.BaseModel `bun:"table:albums,alias:albums"`

	ID      int64 `bun:"id,pk,autoincrement" json:"id"`
	OwnerID int64 `bun:"owner_id,notnull" json:"-"`

	Owner *User `bun:"rel:belongs-to,join:owner_id=id" json:"-"`
}

// Track reaches a user through its album's owner_id.
type Track struct {
	bun.BaseModel `bun:"table:tracks,alias:tracks"`

	ID      int64 `bun:"id,pk,autoincrement" json:"id"`
	AlbumID int64 `bun:"album_id,notnull" json:"albumId"`
}

// Badge embeds a struct from another package, which the index cannot read.
type Badge struct {
	bun.BaseModel `bun:"table:badges,alias:badges"`

	ID int64 `bun:"id,pk,autoincrement" json:"id"`
	audit.Trail
}

// Clip belongs to a model from another package.
type Clip struct {
	bun.BaseModel `bun:"table:clips,alias:clips"`

	ID       int64 `bun:"id,pk,autoincrement" json:"id"`
	SourceID int64 `bun:"source_id,notnull" json:"sourceId"`

	Source *media.Source `bun:"rel:belongs-to,join:source_id=id" json:"-"`
}

// Binder is owned through the mixin and also points at an owned course: its chain must stop at
// its own user_id, not run through course_id.
type Binder struct {
	bun.BaseModel `bun:"table:binders,alias:binders"`

	ID int64 `bun:"id,pk,autoincrement" json:"id"`
	Owned
	CourseID int64 `bun:"course_id,notnull" json:"courseId"`
}

// Record's owner relation has no join: Bun joins owner_id to users.id.
type Record struct {
	bun.BaseModel `bun:"table:records,alias:records"`

	ID      int64 `bun:"id,pk,autoincrement" json:"id"`
	OwnerID int64 `bun:"owner_id,notnull" json:"-"`

	Owner *User `bun:"rel:belongs-to" json:"-"`
}

// Sheet belongs to an owned Folder through holder_id, with Bun's default join.
type Sheet struct {
	bun.BaseModel `bun:"table:sheets,alias:sheets"`

	ID       int64 `bun:"id,pk,autoincrement" json:"id"`
	HolderID int64 `bun:"holder_id,notnull" json:"holderId"`

	Holder *Folder `bun:"rel:belongs-to" json:"-"`
}
