package server

import (
	"handlers"

	// "golang.org/x/net/websocket"
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
	// http.HandleFunc("/messages", handlers.Handler(service.Messages))
	// http.Handle("/ws", websocket.Handler(service.WSServer))
	http.HandleFunc("/ws", handlers.Handler(service.WSHandle))
	http.Handle("/", http.FileServer(http.Dir("public")))
	log.Fatal(http.ListenAndServeTLS(app.Config.Addr, app.Config.Cert, app.Config.Key, nil))
}
