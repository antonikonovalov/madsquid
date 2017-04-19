package server

import (
	"handlers"

	"context"
	"kurento"
	"log"
	"net/http"
)

type App struct {
	Config *Config
}

func New(config *Config) *App {
	return &App{
		Config: config,
	}
}

func (app *App) Run() {
	ctx, cancel := context.WithCancel(context.Background())
	kurentoService, err := kurento.NewService(ctx)
	if err != nil {
		panic(err)
	}
	defer cancel()
	service := handlers.NewService()
	http.HandleFunc("/ws", service.WSHandle)
	http.Handle("/kurento", kurentoService)
	http.HandleFunc("/messages", service.PostMessage)
	http.Handle("/", http.FileServer(http.Dir("public/kurento")))
	if app.Config.TLS {
		log.Fatal(http.ListenAndServeTLS(app.Config.Addr, app.Config.Cert, app.Config.Key, nil))
	} else {
		log.Fatal(http.ListenAndServe(app.Config.Addr, nil))
	}
}
