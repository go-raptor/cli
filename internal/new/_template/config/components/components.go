package components

import (
	"github.com/go-raptor/raptor/v4"
)

func New(c *raptor.Config) *raptor.Components {
	return &raptor.Components{
		Services:    Services(c),
		Middlewares: Middlewares(c),
		Controllers: Controllers(),
	}
}
