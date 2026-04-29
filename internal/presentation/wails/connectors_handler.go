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
	playerResetters []connectorsOnlinePlayerResetter
}

type connectorsTelemetry interface {
	TrackConnectorConnected(ctx context.Context, connectorType string)
}

type connectorsOnlinePlayerResetter interface {
	Reset() error
}

func NewConnectorsHandler(service *service.ConnectorsService, telemetry connectorsTelemetry, playerResetters ...connectorsOnlinePlayerResetter) *ConnectorsHandler {
	return &ConnectorsHandler{service: service, telemetry: telemetry, playerResetters: playerResetters}
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
	}
	return result, nil
}

func (handler *ConnectorsHandler) CancelConnectorConnect(ctx context.Context, request dto.CancelConnectorConnectRequest) error {
	return handler.service.CancelConnectorConnect(ctx, request)
}

func (handler *ConnectorsHandler) GetConnectorConnectSession(ctx context.Context, request dto.GetConnectorConnectSessionRequest) (dto.ConnectorConnectSession, error) {
	return handler.service.GetConnectorConnectSession(ctx, request)
}

func (handler *ConnectorsHandler) OpenConnectorSite(ctx context.Context, request dto.OpenConnectorSiteRequest) error {
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

func isYouTubeConnectorDTO(connector dto.Connector) bool {
	return connector.Type == string(connectors.ConnectorYouTube)
}
