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
	http.HandleFunc("/ws", service.WSHandle)
	http.HandleFunc("/messages", service.PostMessage)
	http.Handle("/", http.FileServer(http.Dir("public")))
	if app.Config.TLS {
		log.Fatal(http.ListenAndServeTLS(app.Config.Addr, app.Config.Cert, app.Config.Key, nil))
	} else {
		log.Fatal(http.ListenAndServe(app.Config.Addr, nil))
	}
}
