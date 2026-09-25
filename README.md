![Raptor](https://static.husak.me/img/raptor/logo.png)

# Raptor CLI

This is the CLI for Raptor. It is heavily inspired by Buffalo CLI, which is a web development eco-system, designed to make the life of a web developer easier. With hot-reload, generators, and other tools, it is a full-stack web development environment.

## Installation

To install the Raptor CLI you can run the following command:

```bash
go install github.com/go-raptor/cli/cmd/raptor@latest
```

## Generated configuration

- **Request body limit.** New projects keep Raptor's 8 MB request body limit. Raise `server.max_body_bytes` only if you accept uploads, and still check each file's size in the upload endpoint.
- **Database password.** `raptor db init postgres` doesn't write a password into `.raptor.dev.yaml` or `.raptor.test.yaml`, because those files are tracked. Set `DATABASE_PASSWORD` in your environment instead.
- **Middleware registration.** `raptor g middleware` creates the middleware but doesn't register it, because its scope (`raptor.Use`, `UseOnly` or `UseExcept`) is your decision. Add it to `config/components/middlewares.go`.
