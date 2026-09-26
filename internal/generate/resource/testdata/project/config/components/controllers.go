package components

import (
	"example.com/shop/app/controllers"
	"github.com/go-raptor/raptor/v4"
)

func Controllers() raptor.Controllers {
	return raptor.Controllers{
		&controllers.AuthController{},
	}
}
