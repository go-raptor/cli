package components

import (
	"github.com/go-raptor/raptor/v4"
	"github.com/go-raptor/template/app/services"
)

func Services(c *raptor.Config) raptor.Services {
	return raptor.Services{
		services.NewHelloService(),
	}
}
