package libraryaccessrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"xiadown/internal/domain/libraryaccess"
)

type SQLiteRepository struct {
	db *bun.DB
}

var _ libraryaccess.Repository = (*SQLiteRepository)(nil)

type configRow struct {
	bun.BaseModel      `bun:"table:library_access_settings"`
	ID                 int    `bun:"id,pk"`
	RemoteEnabled      bool   `bun:"remote_enabled"`
	LANEnabled         bool   `bun:"lan_enabled"`
	LANPort            int    `bun:"lan_port"`
	TailscaleEnabled   bool   `bun:"tailscale_enabled"`
	TailscaleHTTPSPort int    `bun:"tailscale_https_port"`
	TailscalePath      string `bun:"tailscale_path"`
	DeviceName         string `bun:"device_name"`
}

type tailscaleRouteStateRow struct {
	bun.BaseModel      `bun:"table:library_access_tailscale_route_state"`
	ID                 int                                `bun:"id,pk"`
	HTTPSPort          int                                `bun:"https_port"`
	Path               string                             `bun:"route_path"`
	BackendPort        int                                `bun:"backend_port"`
	PendingBackendPort int                                `bun:"pending_backend_port"`
	State              libraryaccess.TailscaleRouteState  `bun:"state"`
	LastAction         libraryaccess.TailscaleRouteAction `bun:"last_action"`
	LastResult         libraryaccess.TailscaleRouteResult `bun:"last_result"`
	LastError          string                             `bun:"last_error"`
	Revision           int64                              `bun:"revision"`
	UpdatedAt          time.Time                          `bun:"updated_at"`
}

func NewSQLiteRepository(db *bun.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (repo *SQLiteRepository) Get(ctx context.Context) (libraryaccess.Config, error) {
	row := new(configRow)
	if err := repo.db.NewSelect().Model(row).Where("id = 1").Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return libraryaccess.Config{}, libraryaccess.ErrConfigNotFound
		}
		return libraryaccess.Config{}, fmt.Errorf("get library access config: %w", err)
	}
	config, err := libraryaccess.NewConfig(libraryaccess.ConfigParams{
		RemoteEnabled: row.RemoteEnabled, LANEnabled: row.LANEnabled, LANPort: row.LANPort,
		TailscaleEnabled: row.TailscaleEnabled, TailscaleHTTPSPort: row.TailscaleHTTPSPort,
		TailscalePath: row.TailscalePath, DeviceName: row.DeviceName,
	})
	if err != nil {
		return libraryaccess.Config{}, fmt.Errorf("decode library access config: %w", err)
	}
	return config, nil
}

func (repo *SQLiteRepository) Save(ctx context.Context, config libraryaccess.Config) error {
	row := &configRow{
		ID: 1, RemoteEnabled: config.RemoteEnabled, LANEnabled: config.LANEnabled,
		LANPort: config.LANPort, TailscaleEnabled: config.TailscaleEnabled,
		TailscaleHTTPSPort: config.TailscaleHTTPSPort, TailscalePath: config.TailscalePath,
		DeviceName: config.DeviceName,
	}
	_, err := repo.db.NewInsert().Model(row).
		On("CONFLICT (id) DO UPDATE").
		Set("remote_enabled = EXCLUDED.remote_enabled").
		Set("lan_enabled = EXCLUDED.lan_enabled").
		Set("lan_port = EXCLUDED.lan_port").
		Set("tailscale_enabled = EXCLUDED.tailscale_enabled").
		Set("tailscale_https_port = EXCLUDED.tailscale_https_port").
		Set("tailscale_path = EXCLUDED.tailscale_path").
		Set("device_name = EXCLUDED.device_name").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("save library access config: %w", err)
	}
	return nil
}

func (repo *SQLiteRepository) GetManagedTailscaleRoute(ctx context.Context) (libraryaccess.ManagedTailscaleRoute, error) {
	row := new(tailscaleRouteStateRow)
	if err := repo.db.NewSelect().Model(row).Where("id = 1").Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return libraryaccess.ManagedTailscaleRoute{}, libraryaccess.ErrManagedTailscaleRouteNotFound
		}
		return libraryaccess.ManagedTailscaleRoute{}, fmt.Errorf("get managed tailscale route: %w", err)
	}
	return managedTailscaleRoute(row), nil
}

// TransitionManagedTailscaleRoute atomically advances the singleton ownership
// record and appends the same event to the immutable audit ledger. The service
// calls it before and after every external `tailscale serve` mutation.
func (repo *SQLiteRepository) TransitionManagedTailscaleRoute(
	ctx context.Context,
	transition libraryaccess.TailscaleRouteTransition,
) (libraryaccess.ManagedTailscaleRoute, error) {
	normalized, err := libraryaccess.NewTailscaleRouteTransition(
		transition.HTTPSPort,
		transition.Path,
		transition.BackendPort,
		transition.PendingBackendPort,
		transition.State,
		transition.Action,
		transition.Result,
		transition.Error,
	)
	if err != nil {
		return libraryaccess.ManagedTailscaleRoute{}, err
	}

	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return libraryaccess.ManagedTailscaleRoute{}, fmt.Errorf("begin managed tailscale route transition: %w", err)
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
INSERT INTO library_access_tailscale_route_state (
    id, https_port, route_path, backend_port, pending_backend_port,
    state, last_action, last_result,
    last_error, revision, updated_at
)
VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?)
ON CONFLICT (id) DO UPDATE SET
    https_port = EXCLUDED.https_port,
    route_path = EXCLUDED.route_path,
	backend_port = EXCLUDED.backend_port,
	pending_backend_port = EXCLUDED.pending_backend_port,
    state = EXCLUDED.state,
    last_action = EXCLUDED.last_action,
    last_result = EXCLUDED.last_result,
    last_error = EXCLUDED.last_error,
    revision = library_access_tailscale_route_state.revision + 1,
    updated_at = EXCLUDED.updated_at
`, normalized.HTTPSPort, normalized.Path, normalized.BackendPort, normalized.PendingBackendPort,
		normalized.State, normalized.Action, normalized.Result, normalized.Error, now); err != nil {
		return libraryaccess.ManagedTailscaleRoute{}, fmt.Errorf("update managed tailscale route: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO library_access_tailscale_route_audit (
    https_port, route_path, backend_port, pending_backend_port,
    state, action, result, error, transitioned_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
`, normalized.HTTPSPort, normalized.Path, normalized.BackendPort, normalized.PendingBackendPort,
		normalized.State, normalized.Action, normalized.Result, normalized.Error, now); err != nil {
		return libraryaccess.ManagedTailscaleRoute{}, fmt.Errorf("audit managed tailscale route transition: %w", err)
	}

	row := new(tailscaleRouteStateRow)
	if err := tx.NewSelect().Model(row).Where("id = 1").Scan(ctx); err != nil {
		return libraryaccess.ManagedTailscaleRoute{}, fmt.Errorf("read managed tailscale route transition: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return libraryaccess.ManagedTailscaleRoute{}, fmt.Errorf("commit managed tailscale route transition: %w", err)
	}
	return managedTailscaleRoute(row), nil
}

func managedTailscaleRoute(row *tailscaleRouteStateRow) libraryaccess.ManagedTailscaleRoute {
	return libraryaccess.ManagedTailscaleRoute{
		HTTPSPort:          row.HTTPSPort,
		Path:               row.Path,
		BackendPort:        row.BackendPort,
		PendingBackendPort: row.PendingBackendPort,
		State:              row.State,
		LastAction:         row.LastAction,
		LastResult:         row.LastResult,
		LastError:          row.LastError,
		Revision:           row.Revision,
		UpdatedAt:          row.UpdatedAt,
	}
}
