package server

import (
	"http"

	"golang.org/x/net/context"
)

type App struct {
	Config *Config
}

func New() *App {
	config = GetConfig()

	return &App{
		Config: config,
	}
}

func (app *App) Run() {

}

type ctxHandler func(context.Context, http.ResponseWriter, *http.Request)

func (app *App) Handler(h ctxHandler) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		ctx := context.Background()
		ctx = SetUserName(ctx, req)

		h(ctx, rw, req)
	}
}

func SetUserName(ctx context.Context, req *http.Request) context.Context {
	return context.WithValue(ctx, "user", req.Header.Get("X-User-Name"))
}
