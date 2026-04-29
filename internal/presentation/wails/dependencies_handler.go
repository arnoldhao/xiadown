package wails

import (
	"context"
	"strings"

	"xiadown/internal/application/dependencies/dto"
	"xiadown/internal/application/dependencies/service"
	"xiadown/internal/domain/dependencies"
	"xiadown/internal/infrastructure/opener"
)

type DependenciesHandler struct {
	service   *service.DependenciesService
	events    dependenciesEvents
	telemetry dependenciesTelemetry
}

type dependenciesEvents interface {
	EmitDependenciesUpdated()
}

type dependenciesTelemetry interface {
	TrackDependencyInstalled(ctx context.Context, dependencyName string)
}

func NewDependenciesHandler(service *service.DependenciesService, events dependenciesEvents, telemetry dependenciesTelemetry) *DependenciesHandler {
	return &DependenciesHandler{
		service:   service,
		events:    events,
		telemetry: telemetry,
	}
}

func (handler *DependenciesHandler) ServiceName() string {
	return "DependenciesHandler"
}

func (handler *DependenciesHandler) ListDependencies(ctx context.Context) ([]dto.Dependency, error) {
	return handler.service.ListDependencies(ctx)
}

func (handler *DependenciesHandler) ListDependencyUpdates(ctx context.Context) ([]dto.DependencyUpdateInfo, error) {
	return handler.service.ListDependencyUpdates(ctx)
}

func (handler *DependenciesHandler) GetDependencyInstallState(ctx context.Context, request dto.GetDependencyInstallStateRequest) (dto.DependencyInstallState, error) {
	return handler.service.GetInstallState(ctx, request)
}

func (handler *DependenciesHandler) InstallDependency(ctx context.Context, request dto.InstallDependencyRequest) (dto.Dependency, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return dto.Dependency{}, dependencies.ErrInvalidDependency
	}
	if state, err := handler.service.GetInstallState(ctx, dto.GetDependencyInstallStateRequest{Name: name}); err == nil {
		switch strings.TrimSpace(state.Stage) {
		case "downloading", "extracting", "verifying":
			return dto.Dependency{Name: name}, nil
		}
	}
	go func(request dto.InstallDependencyRequest) {
		result, err := handler.service.InstallDependency(context.Background(), request)
		if handler.events != nil {
			handler.events.EmitDependenciesUpdated()
		}
		if err == nil && handler.telemetry != nil {
			handler.telemetry.TrackDependencyInstalled(context.Background(), result.Name)
		}
	}(request)
	return dto.Dependency{Name: name}, nil
}

func (handler *DependenciesHandler) VerifyDependency(ctx context.Context, request dto.VerifyDependencyRequest) (dto.Dependency, error) {
	result, err := handler.service.VerifyDependency(ctx, request)
	if err == nil && handler.events != nil {
		handler.events.EmitDependenciesUpdated()
	}
	return result, err
}

func (handler *DependenciesHandler) SetDependencyPath(ctx context.Context, request dto.SetDependencyPathRequest) (dto.Dependency, error) {
	result, err := handler.service.SetDependencyPath(ctx, request)
	if err == nil && handler.events != nil {
		handler.events.EmitDependenciesUpdated()
	}
	return result, err
}

func (handler *DependenciesHandler) RemoveDependency(ctx context.Context, request dto.RemoveDependencyRequest) error {
	if err := handler.service.RemoveDependency(ctx, request); err != nil {
		return err
	}
	if handler.events != nil {
		handler.events.EmitDependenciesUpdated()
	}
	return nil
}

func (handler *DependenciesHandler) OpenDependencyDirectory(ctx context.Context, request dto.OpenDependencyDirectoryRequest) error {
	name := dependencies.DependencyName(request.Name)
	if name == "" {
		return dependencies.ErrInvalidDependency
	}
	dir, err := handler.service.ResolveDependencyDirectory(ctx, name)
	if err != nil {
		return err
	}
	return opener.OpenDirectory(dir)
}
