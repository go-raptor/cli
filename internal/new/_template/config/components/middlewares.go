package components

import (
	"log/slog"
	"os"

	"github.com/go-raptor/middlewares/cors"
	"github.com/go-raptor/middlewares/logger"
	"github.com/go-raptor/raptor/v4"
	"github.com/go-raptor/raptor/v4/core"
	"github.com/lmittmann/tint"
)

func Middlewares() raptor.Middlewares {
	return raptor.Middlewares{
		core.Use(logger.New(func(level *slog.LevelVar) slog.Handler {
			return tint.NewHandler(os.Stderr, &tint.Options{Level: level})
		})),
		core.Use(&cors.CORSMiddleware{}),
	}
}
