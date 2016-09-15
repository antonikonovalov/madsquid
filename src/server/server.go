package server

import (
	"handlers"

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
	service := handlers.NewService()
	http.HandleFunc("/ws", handlers.Handler(service.WSHandle))
	http.Handle("/", http.FileServer(http.Dir("public")))
	log.Fatal(http.ListenAndServeTLS(app.Config.Addr, app.Config.Cert, app.Config.Key, nil))
}
