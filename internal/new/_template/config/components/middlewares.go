package components

import (
	"github.com/go-raptor/middlewares/csrf"
	"github.com/go-raptor/middlewares/logger"
	"github.com/go-raptor/middlewares/requestid"
	"github.com/go-raptor/middlewares/secure"
	"github.com/go-raptor/raptor/v4"
)

func Middlewares() raptor.Middlewares {
	return raptor.Middlewares{
		// First, so the request log and Raptor's error lines carry the request_id.
		raptor.Use(&requestid.RequestIDMiddleware{}),
		raptor.Use(&logger.LoggerMiddleware{}),
		// Security headers on every response, error responses included. HSTS stays off until
		// app.secure_hsts_max_age is set; see README.md.
		raptor.Use(&secure.SecureMiddleware{}),
		// Rejects cross-origin writes from browsers. A frontend on another origin needs
		// app.csrf_trusted_origins and the cors middleware; see README.md.
		raptor.Use(&csrf.CSRFMiddleware{}),
	}
}
