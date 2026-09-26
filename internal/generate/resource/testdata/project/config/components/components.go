package components

import (
	"example.com/shop/db"
	"github.com/go-raptor/connectors/bun/postgres"
	"github.com/go-raptor/raptor/v4"
)

func New() *raptor.Components {
	return &raptor.Components{
		DatabaseConnector: postgres.NewPostgresConnector(db.MigrationsFS()),
		Services:          Services(),
		Controllers:       Controllers(),
		Middlewares:       Middlewares(),
	}
}
