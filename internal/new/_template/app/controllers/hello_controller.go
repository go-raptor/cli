package controllers

import (
	"net/http"

	"github.com/go-raptor/raptor/v4"
	"github.com/go-raptor/raptor/v4/errs"
	"github.com/go-raptor/template/app/services"
)

type HelloController struct {
	raptor.Controller

	Hello *services.HelloService
}

func (c *HelloController) Greet(ctx *raptor.Context) error {
	greeting := map[string]any{
		"message":   c.Hello.Greeting(),
		"greetings": c.Hello.Greetings(),
	}

	return ctx.Data(greeting)
}

func (c *HelloController) AddGreetings(ctx *raptor.Context) error {
	var request struct {
		Greeting string `json:"greeting"`
	}

	if err := ctx.Bind(&request); err != nil {
		return errs.NewErrorBadRequest("invalid request")
	}

	c.Hello.AddGreeting(request.Greeting)

	return ctx.Status(http.StatusCreated)
}
