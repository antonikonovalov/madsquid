package handlers

import (
	"context"
	"net/http"
	"sync"
	"ws"
)

type Service struct {
	sync.Mutex
	clients map[string]*ws.WS
}

func NewService() *Service {
	return &Service{
		clients: map[string]*ws.WS{},
	}
}

type ctxHandler func(context.Context, http.ResponseWriter, *http.Request)

func Handler(h ctxHandler) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		ctx := context.Background()
		ctx = SetUserName(ctx, req)

		h(ctx, rw, req)
	}
}

func SetUserName(ctx context.Context, req *http.Request) context.Context {
	return context.WithValue(ctx, "user", req.FormValue("user"))
}

func GetUserName(ctx context.Context) string {
	name, _ := ctx.Value("user").(string)
	return name
}
