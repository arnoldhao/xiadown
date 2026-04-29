package wails

import (
	"context"

	fontsservice "xiadown/internal/application/fonts/service"
	"xiadown/internal/infrastructure/opener"
)

type SystemHandler struct {
	fonts *fontsservice.FontService
}

func NewSystemHandler(fonts *fontsservice.FontService) *SystemHandler {
	return &SystemHandler{fonts: fonts}
}

func (handler *SystemHandler) ServiceName() string {
	return "SystemHandler"
}

func (handler *SystemHandler) ListFontFamilies(ctx context.Context) ([]string, error) {
	if handler.fonts == nil {
		return []string{}, nil
	}
	families, err := handler.fonts.ListFontFamilies(ctx)
	if err != nil {
		return nil, err
	}
	if families == nil {
		return []string{}, nil
	}
	return families, nil
}

type OpenURLRequest struct {
	URL string `json:"url"`
}

func (handler *SystemHandler) OpenURL(_ context.Context, request OpenURLRequest) error {
	return opener.OpenURL(request.URL)
}

type CurrentUserProfile struct {
	Username     string `json:"username"`
	DisplayName  string `json:"displayName"`
	Initials     string `json:"initials,omitempty"`
	AvatarPath   string `json:"avatarPath,omitempty"`
	AvatarBase64 string `json:"avatarBase64,omitempty"`
	AvatarMime   string `json:"avatarMime,omitempty"`
}

func (handler *SystemHandler) GetCurrentUserProfile(ctx context.Context) (CurrentUserProfile, error) {
	return loadCurrentUserProfile(ctx)
}
