package components

import (
	"github.com/go-raptor/middlewares/csrf"
	"github.com/go-raptor/middlewares/logger"
	"github.com/go-raptor/raptor/v4"
)

func Middlewares() raptor.Middlewares {
	return raptor.Middlewares{
		raptor.Use(&logger.LoggerMiddleware{}),
		// Rejects cross-origin writes from browsers. A frontend on another origin needs
		// app.csrf_trusted_origins and the cors middleware; see README.md.
		raptor.Use(&csrf.CSRFMiddleware{}),
	}
}
