package main

import (
	"github.com/go-raptor/raptor/v4"
	"github.com/go-raptor/template/config"
	"github.com/go-raptor/template/config/components"
)

func main() {
	app := raptor.New()
	app.Configure(components.New(app.Core.Resources.Config))
	app.RegisterRoutes(config.Routes())
	app.Run()
}
