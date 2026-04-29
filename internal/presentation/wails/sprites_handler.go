package wails

import (
	"context"

	"xiadown/internal/application/sprites/dto"
	"xiadown/internal/application/sprites/service"
)

type SpritesHandler struct {
	service *service.Service
}

func NewSpritesHandler(service *service.Service) *SpritesHandler {
	return &SpritesHandler{service: service}
}

func (handler *SpritesHandler) ServiceName() string {
	return "SpritesHandler"
}

func (handler *SpritesHandler) ListSprites(ctx context.Context) ([]dto.Sprite, error) {
	return handler.service.ListSprites(ctx)
}

func (handler *SpritesHandler) InspectSpriteSource(ctx context.Context, request dto.InspectSpriteRequest) (dto.SpriteImportDraft, error) {
	return handler.service.InspectSpriteSource(ctx, request)
}

func (handler *SpritesHandler) ImportSprite(ctx context.Context, request dto.ImportSpriteRequest) (dto.Sprite, error) {
	return handler.service.ImportSprite(ctx, request)
}

func (handler *SpritesHandler) InstallSpriteFromURL(ctx context.Context, request dto.InstallSpriteFromURLRequest) (dto.Sprite, error) {
	return handler.service.InstallSpriteFromURL(ctx, request)
}

func (handler *SpritesHandler) UpdateSprite(ctx context.Context, request dto.UpdateSpriteRequest) (dto.Sprite, error) {
	return handler.service.UpdateSprite(ctx, request)
}

func (handler *SpritesHandler) GetSpriteManifest(ctx context.Context, request dto.GetSpriteManifestRequest) (dto.SpriteManifest, error) {
	return handler.service.GetSpriteManifest(ctx, request)
}

func (handler *SpritesHandler) UpdateSpriteSlices(ctx context.Context, request dto.UpdateSpriteSlicesRequest) (dto.Sprite, error) {
	return handler.service.UpdateSpriteSlices(ctx, request)
}

func (handler *SpritesHandler) ExportSprite(ctx context.Context, request dto.ExportSpriteRequest) error {
	return handler.service.ExportSprite(ctx, request)
}

func (handler *SpritesHandler) DeleteSprite(ctx context.Context, request dto.DeleteSpriteRequest) error {
	return handler.service.DeleteSprite(ctx, request)
}
