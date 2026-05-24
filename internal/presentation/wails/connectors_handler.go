package wails

import (
	"context"
	"strings"

	"xiadown/internal/application/connectors/dto"
	"xiadown/internal/application/connectors/service"
	"xiadown/internal/domain/connectors"
)

type ConnectorsHandler struct {
	service         *service.ConnectorsService
	telemetry       connectorsTelemetry
	windows         *WindowManager
	playerResetters []connectorsOnlinePlayerResetter
}

const ListenYouTubeConnectorChangedEvent = "listen:youtube-connector:changed"

type connectorsTelemetry interface {
	TrackConnectorConnected(ctx context.Context, connectorType string)
}

type connectorsOnlinePlayerResetter interface {
	Reset() error
}

func NewConnectorsHandler(service *service.ConnectorsService, windows *WindowManager, telemetry connectorsTelemetry, playerResetters ...connectorsOnlinePlayerResetter) *ConnectorsHandler {
	return &ConnectorsHandler{service: service, telemetry: telemetry, windows: windows, playerResetters: playerResetters}
}

func (handler *ConnectorsHandler) ServiceName() string {
	return "ConnectorsHandler"
}

func (handler *ConnectorsHandler) ListConnectors(ctx context.Context) ([]dto.Connector, error) {
	return handler.service.ListConnectors(ctx)
}

func (handler *ConnectorsHandler) UpsertConnector(ctx context.Context, request dto.UpsertConnectorRequest) (dto.Connector, error) {
	connector, err := handler.service.UpsertConnector(ctx, request)
	if err != nil {
		return dto.Connector{}, err
	}
	if isYouTubeConnectorDTO(connector) {
		handler.resetOnlinePlayer()
		handler.dispatchYouTubeConnectorChanged("upsert", connector.ID, connector.Status)
	}
	return connector, nil
}

func (handler *ConnectorsHandler) ClearConnector(ctx context.Context, request dto.ClearConnectorRequest) error {
	resetAfterClear := handler.connectorIDHasType(ctx, request.ID, connectors.ConnectorYouTube)
	if err := handler.service.ClearConnector(ctx, request); err != nil {
		return err
	}
	if resetAfterClear {
		handler.resetOnlinePlayer()
		handler.dispatchYouTubeConnectorChanged("clear", request.ID, connectors.StatusDisconnected)
	}
	return nil
}

func (handler *ConnectorsHandler) StartConnectorConnect(ctx context.Context, request dto.StartConnectorConnectRequest) (dto.StartConnectorConnectResult, error) {
	resetBeforeStart := handler.connectorIDHasType(ctx, request.ID, connectors.ConnectorYouTube)
	if resetBeforeStart {
		handler.resetOnlinePlayer()
	}
	result, err := handler.service.StartConnectorConnect(ctx, request)
	if err != nil {
		return dto.StartConnectorConnectResult{}, err
	}
	if !resetBeforeStart && isYouTubeConnectorDTO(result.Connector) {
		handler.resetOnlinePlayer()
	}
	return result, nil
}

func (handler *ConnectorsHandler) FinishConnectorConnect(ctx context.Context, request dto.FinishConnectorConnectRequest) (dto.FinishConnectorConnectResult, error) {
	result, err := handler.service.FinishConnectorConnect(ctx, request)
	if err != nil {
		return dto.FinishConnectorConnectResult{}, err
	}
	if handler.telemetry != nil && result.Saved && result.Connector.Status == "connected" {
		handler.telemetry.TrackConnectorConnected(ctx, result.Connector.Type)
	}
	if result.Saved && isYouTubeConnectorDTO(result.Connector) {
		handler.resetOnlinePlayer()
		handler.dispatchYouTubeConnectorChanged("finish", result.Connector.ID, result.Connector.Status)
	}
	return result, nil
}

func (handler *ConnectorsHandler) CancelConnectorConnect(ctx context.Context, request dto.CancelConnectorConnectRequest) error {
	return handler.service.CancelConnectorConnect(ctx, request)
}

func (handler *ConnectorsHandler) GetConnectorConnectSession(ctx context.Context, request dto.GetConnectorConnectSessionRequest) (dto.ConnectorConnectSession, error) {
	return handler.service.GetConnectorConnectSession(ctx, request)
}

func (handler *ConnectorsHandler) OpenConnectorSite(ctx context.Context, request dto.OpenConnectorSiteRequest) (dto.StartConnectorConnectResult, error) {
	return handler.service.OpenConnectorSite(ctx, request)
}

func (handler *ConnectorsHandler) connectorIDHasType(ctx context.Context, id string, connectorType connectors.ConnectorType) bool {
	id = strings.TrimSpace(id)
	if id == "" || handler == nil || handler.service == nil {
		return false
	}
	items, err := handler.service.ListConnectors(ctx)
	if err != nil {
		return id == "connector-youtube" && connectorType == connectors.ConnectorYouTube
	}
	for _, item := range items {
		if strings.TrimSpace(item.ID) == id && item.Type == string(connectorType) {
			return true
		}
	}
	return false
}

func (handler *ConnectorsHandler) resetOnlinePlayer() {
	if handler == nil {
		return
	}
	for _, resetter := range handler.playerResetters {
		if resetter != nil {
			_ = resetter.Reset()
		}
	}
}

func (handler *ConnectorsHandler) dispatchYouTubeConnectorChanged(action string, connectorID string, status any) {
	if handler == nil || handler.windows == nil {
		return
	}
	handler.windows.dispatchWindowEvent(ListenYouTubeConnectorChangedEvent, map[string]any{
		"action":        strings.TrimSpace(action),
		"connectorId":   strings.TrimSpace(connectorID),
		"connectorType": string(connectors.ConnectorYouTube),
		"status":        strings.TrimSpace(toConnectorStatusString(status)),
	})
}

func toConnectorStatusString(status any) string {
	switch value := status.(type) {
	case connectors.ConnectorStatus:
		return string(value)
	case string:
		return value
	default:
		return ""
	}
}

func isYouTubeConnectorDTO(connector dto.Connector) bool {
	return connector.Type == string(connectors.ConnectorYouTube)
}
