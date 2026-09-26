package controllers

import (
	"github.com/go-raptor/raptor/v4"
	"github.com/go-raptor/raptor/v4/errs"
)

type AuthController struct {
	raptor.Controller
}

// Login stands in for the skill's session login; the fixture only has to compile.
func (c *AuthController) Login(ctx *raptor.Context) error {
	return errs.ErrUnauthorized
}
