// Package libraryapi exposes the isolated, versioned Library public API.
// The router is intentionally built from a fresh ServeMux: it cannot inherit
// desktop handlers such as /api/library/asset?path= or /ws.
package libraryapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"

	"xiadown/internal/application/library/access"
	"xiadown/internal/domain/library"
)

const maxPairingBodyBytes = 32 << 10

type Authenticator interface {
	Authenticate(ctx context.Context, token string) (access.Principal, error)
}

type Pairer interface {
	Pair(ctx context.Context, request access.PairRequest) (access.PairResult, error)
}

type ProtectedRoute struct {
	Method  string
	Path    string
	Scope   library.DeviceScope
	Handler http.Handler
}

// AuthenticatedRoute requires a catalog-bound bearer token but deliberately
// does not require a business-data scope. It is reserved for safe credential
// introspection such as discovering the current grant's effective scopes.
type AuthenticatedRoute struct {
	Method  string
	Path    string
	Handler http.Handler
}

type Config struct {
	Version       string
	CatalogID     string
	Authenticator Authenticator
	Pairer        Pairer
	// Capabilities advertises shared control/signal-plane protocols that do not
	// belong to one Station, such as the scope-filtered synchronization SSE.
	Capabilities []string
	// StationCapabilities advertises only device-facing APIs that are actually
	// mounted in this router. Scope presence alone must not make an older
	// Desktop look Music-capable to iOS.
	StationCapabilities map[string][]string
	AuthenticatedRoutes []AuthenticatedRoute
	Routes              []ProtectedRoute
}

type principalContextKey struct{}

func NewRouter(config Config) (http.Handler, error) {
	config.CatalogID = strings.TrimSpace(config.CatalogID)
	if config.CatalogID == "" || config.Authenticator == nil || config.Pairer == nil {
		return nil, errors.New("library public API requires authentication and pairing services")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "api": 1})
	})
	mux.HandleFunc("GET /api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"api": 1, "version": strings.TrimSpace(config.Version)})
	})
	mux.Handle("POST /api/v1/pair", pairHandler(config.Pairer))

	authenticatedRoutes := append(deviceAccessRoutes(config.Capabilities, config.StationCapabilities), config.AuthenticatedRoutes...)
	for _, route := range authenticatedRoutes {
		method := strings.ToUpper(strings.TrimSpace(route.Method))
		path := strings.TrimSpace(route.Path)
		if method == "" || !strings.HasPrefix(path, "/api/v1/") || route.Handler == nil {
			return nil, errors.New("invalid Library public API authenticated route")
		}
		mux.Handle(method+" "+path, authenticate(config.Authenticator, config.CatalogID, route.Handler))
	}

	for _, route := range config.Routes {
		method := strings.ToUpper(strings.TrimSpace(route.Method))
		path := strings.TrimSpace(route.Path)
		if method == "" || !strings.HasPrefix(path, "/api/v1/") || route.Scope == "" || route.Handler == nil {
			return nil, errors.New("invalid Library public API protected route")
		}
		mux.Handle(method+" "+path, authenticate(config.Authenticator, config.CatalogID, requireScope(route.Scope, route.Handler)))
	}
	mux.Handle("/api/v1/", authenticate(config.Authenticator, config.CatalogID, http.HandlerFunc(http.NotFound)))

	// Only /api/v1 participates in authentication. Every desktop/internal path
	// is an unconditional 404 even if a valid public API token is supplied.
	return securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if !strings.HasPrefix(request.URL.Path, "/api/v1/") {
			http.NotFound(w, request)
			return
		}
		mux.ServeHTTP(w, request)
	})), nil
}

type deviceAccessStationSummary struct {
	Supported    bool     `json:"supported"`
	Authorized   bool     `json:"authorized"`
	Capabilities []string `json:"capabilities"`
}

func deviceAccessRoutes(capabilities []string, stationCapabilities map[string][]string) []AuthenticatedRoute {
	return []AuthenticatedRoute{{
		Method: http.MethodGet,
		Path:   "/api/v1/device-access",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			principal, ok := PrincipalFromContext(request.Context())
			if !ok {
				unauthorized(w)
				return
			}
			writeJSON(w, http.StatusOK, struct {
				GrantID      string                                `json:"grantId"`
				CatalogID    string                                `json:"catalogId"`
				DeviceID     string                                `json:"deviceId"`
				Scopes       []library.DeviceScope                 `json:"scopes"`
				Capabilities []string                              `json:"capabilities,omitempty"`
				Stations     map[string]deviceAccessStationSummary `json:"stations,omitempty"`
			}{
				GrantID: principal.GrantID, CatalogID: principal.CatalogID,
				DeviceID: principal.DeviceID, Scopes: append([]library.DeviceScope(nil), principal.Scopes...),
				Capabilities: normalizedCapabilities(capabilities),
				Stations:     deviceAccessStations(principal, stationCapabilities),
			})
		}),
	}}
}

func normalizedCapabilities(capabilities []string) []string {
	unique := make(map[string]struct{}, len(capabilities))
	for _, capability := range capabilities {
		if capability = strings.TrimSpace(capability); capability != "" {
			unique[capability] = struct{}{}
		}
	}
	if len(unique) == 0 {
		return nil
	}
	result := make([]string, 0, len(unique))
	for capability := range unique {
		result = append(result, capability)
	}
	sort.Strings(result)
	return result
}

func deviceAccessStations(
	principal access.Principal,
	stationCapabilities map[string][]string,
) map[string]deviceAccessStationSummary {
	// Authorized means the grant may enter the Station's read surface. Narrow
	// mutation/fetch scopes remain independently visible in the top-level scope
	// set and must never imply read access.
	readScopes := map[string]library.DeviceScope{
		"library": library.DeviceScopeLibraryRead,
		"music":   library.DeviceScopeMusicRead,
		"rss":     library.DeviceScopeRSSRead,
	}
	result := make(map[string]deviceAccessStationSummary, len(stationCapabilities))
	for _, station := range []string{"library", "music", "rss"} {
		capabilities, supported := stationCapabilities[station]
		if !supported {
			continue
		}
		ordered := normalizedCapabilities(capabilities)
		result[station] = deviceAccessStationSummary{
			Supported: true, Authorized: principal.HasScope(readScopes[station]), Capabilities: ordered,
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func pairHandler(pairer Pairer) http.Handler {
	type pairPayload struct {
		Nonce         string `json:"nonce"`
		Code          string `json:"code"`
		DeviceID      string `json:"deviceID"`
		Name          string `json:"name"`
		PublicKeyHash string `json:"publicKeyHash"`
	}
	type pairResponse struct {
		GrantID string                `json:"grantID"`
		Token   string                `json:"token"`
		Scopes  []library.DeviceScope `json:"scopes"`
	}
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		request.Body = http.MaxBytesReader(w, request.Body, maxPairingBodyBytes)
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		var payload pairPayload
		if err := decoder.Decode(&payload); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request")
			return
		}
		if err := ensureJSONEOF(decoder); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request")
			return
		}
		result, err := pairer.Pair(request.Context(), access.PairRequest{
			Nonce: payload.Nonce, Code: payload.Code, DeviceID: payload.DeviceID,
			DeviceName: payload.Name, PublicKeyHash: payload.PublicKeyHash,
		})
		if err != nil {
			status := http.StatusInternalServerError
			code := "pairing_failed"
			if errors.Is(err, access.ErrInvalidRequest) {
				status, code = http.StatusBadRequest, "invalid_request"
			} else if errors.Is(err, access.ErrPairingInvalid) {
				status, code = http.StatusUnauthorized, "pairing_invalid"
			}
			writeError(w, status, code)
			return
		}
		writeJSON(w, http.StatusCreated, pairResponse{GrantID: result.GrantID, Token: result.Token, Scopes: result.Scopes})
	})
}

func authenticate(authenticator Authenticator, catalogID string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		token, ok := bearerToken(request.Header.Get("Authorization"))
		if !ok {
			unauthorized(w)
			return
		}
		principal, err := authenticator.Authenticate(request.Context(), token)
		if err != nil {
			unauthorized(w)
			return
		}
		// Device grants are catalog-scoped. Scope names alone must never let a
		// valid token issued for another Catalog cross into this public server.
		if strings.TrimSpace(principal.CatalogID) != catalogID {
			writeError(w, http.StatusForbidden, "catalog_access_denied")
			return
		}
		ctx := context.WithValue(request.Context(), principalContextKey{}, principal)
		next.ServeHTTP(w, request.WithContext(ctx))
	})
}

func requireScope(scope library.DeviceScope, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		principal, ok := PrincipalFromContext(request.Context())
		if !ok || !principal.HasScope(scope) {
			writeError(w, http.StatusForbidden, "insufficient_scope")
			return
		}
		next.ServeHTTP(w, request)
	})
}

func PrincipalFromContext(ctx context.Context) (access.Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(access.Principal)
	return principal, ok
}

func bearerToken(header string) (string, bool) {
	parts := strings.Fields(header)
	returnValue := ""
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		returnValue = parts[1]
	}
	return returnValue, returnValue != ""
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="xiadown-library"`)
	writeError(w, http.StatusUnauthorized, "unauthorized")
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, request)
	})
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("multiple JSON values")
	}
	return err
}
