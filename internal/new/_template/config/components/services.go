package components

import (
	"github.com/go-raptor/raptor/v4"
	"github.com/go-raptor/template/app/services"
)

func Services() raptor.Services {
	return raptor.Services{
		&services.HelloService{},
	}
}
