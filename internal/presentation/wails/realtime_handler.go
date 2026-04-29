package wails

import (
	"context"

	"xiadown/internal/infrastructure/ws"
)

type RealtimeHandler struct {
	server *ws.Server
}

func NewRealtimeHandler(server *ws.Server) *RealtimeHandler {
	return &RealtimeHandler{server: server}
}

func (handler *RealtimeHandler) ServiceName() string {
	return "RealtimeHandler"
}

func (handler *RealtimeHandler) WebSocketURL(_ context.Context) (string, error) {
	return handler.server.URL(), nil
}

func (handler *RealtimeHandler) HTTPBaseURL(_ context.Context) (string, error) {
	return handler.server.HTTPURL(), nil
}
