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
	http.HandleFunc("/messages", handlers.Handler(service.Messages))
	http.Handle("/", http.FileServer(http.Dir("public")))
	log.Fatal(http.ListenAndServe(app.Config.Addr, nil))
}
