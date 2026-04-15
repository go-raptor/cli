package controllers_test

import (
	"os"
	"testing"

	"github.com/go-raptor/raptor/v4"
	"github.com/go-raptor/template/config"
	"github.com/go-raptor/template/config/components"
)

var app *raptor.Raptor

func TestMain(m *testing.M) {
	app = raptor.NewTestApp(components.New(), config.Routes())
	os.Exit(m.Run())
}
