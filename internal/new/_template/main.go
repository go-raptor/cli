package main

import (
	"github.com/go-raptor/raptor/v4"
	"github.com/go-raptor/template/config"
	"github.com/go-raptor/template/config/components"
)

func main() {
	raptor.New(components.New(), config.Routes()).Run()
}
