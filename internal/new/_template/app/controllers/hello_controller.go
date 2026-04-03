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

func (hc *HelloController) Greet(c *raptor.Context) error {
	greeting := map[string]interface{}{
		"message":   hc.Hello.Greeting(),
		"greetings": hc.Hello.Greetings(),
	}

	return c.Data(greeting)
}

func (hc *HelloController) AddGreetings(c *raptor.Context) error {
	var request struct {
		Greeting string `json:"greeting"`
	}

	if err := c.Bind(&request); err != nil {
		return errs.NewErrorBadRequest("invalid request")
	}

	hc.Hello.AddGreeting(request.Greeting)

	return c.Status(http.StatusCreated)
}
