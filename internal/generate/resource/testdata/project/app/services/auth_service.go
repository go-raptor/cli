package services

import (
	"example.com/shop/app/models"
	"github.com/go-raptor/raptor/v4"
	"github.com/go-raptor/raptor/v4/errs"
)

type AuthService struct {
	raptor.Service
}

// CurrentUser stands in for the session lookup in the skill's auth.md; the fixture only has to
// compile.
func (s *AuthService) CurrentUser(ctx *raptor.Context) (*models.User, error) {
	user, ok := ctx.Get("user").(*models.User)
	if !ok {
		return nil, errs.ErrUnauthorized
	}
	return user, nil
}
