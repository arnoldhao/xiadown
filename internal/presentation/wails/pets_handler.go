package wails

import (
	"context"

	"xiadown/internal/application/pets/dto"
	"xiadown/internal/application/pets/service"
)

type PetsHandler struct {
	service *service.Service
}

func NewPetsHandler(service *service.Service) *PetsHandler {
	return &PetsHandler{service: service}
}

func (handler *PetsHandler) ServiceName() string {
	return "PetsHandler"
}

func (handler *PetsHandler) ListPets(ctx context.Context) ([]dto.Pet, error) {
	return handler.service.ListPets(ctx)
}

func (handler *PetsHandler) InspectPetSource(ctx context.Context, request dto.InspectPetRequest) (dto.PetImportDraft, error) {
	return handler.service.InspectPetSource(ctx, request)
}

func (handler *PetsHandler) ImportPet(ctx context.Context, request dto.ImportPetRequest) (dto.Pet, error) {
	return handler.service.ImportPet(ctx, request)
}

func (handler *PetsHandler) StartOnlinePetImport(ctx context.Context, request dto.StartOnlinePetImportRequest) (dto.OnlinePetImportSession, error) {
	return handler.service.StartOnlinePetImport(ctx, request)
}

func (handler *PetsHandler) GetOnlinePetImportSession(ctx context.Context, request dto.GetOnlinePetImportSessionRequest) (dto.OnlinePetImportSession, error) {
	return handler.service.GetOnlinePetImportSession(ctx, request)
}

func (handler *PetsHandler) FinishOnlinePetImportSession(ctx context.Context, request dto.FinishOnlinePetImportSessionRequest) (dto.OnlinePetImportSession, error) {
	return handler.service.FinishOnlinePetImportSession(ctx, request)
}

func (handler *PetsHandler) GetPetManifest(ctx context.Context, request dto.GetPetManifestRequest) (dto.PetManifest, error) {
	return handler.service.GetPetManifest(ctx, request)
}

func (handler *PetsHandler) ExportPet(ctx context.Context, request dto.ExportPetRequest) error {
	return handler.service.ExportPet(ctx, request)
}

func (handler *PetsHandler) DeletePet(ctx context.Context, request dto.DeletePetRequest) error {
	return handler.service.DeletePet(ctx, request)
}
