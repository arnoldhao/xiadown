package libraryrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/uptrace/bun"

	"xiadown/internal/domain/library"
)

const moduleConfigRowID = 1

type SQLiteModuleConfigRepository struct {
	db *bun.DB
}

type moduleConfigRow struct {
	bun.BaseModel `bun:"table:library_module_config"`
	ID            int       `bun:"id,pk"`
	ConfigJSON    string    `bun:"config_json"`
	UpdatedAt     time.Time `bun:"updated_at"`
}

func NewSQLiteModuleConfigRepository(db *bun.DB) *SQLiteModuleConfigRepository {
	return &SQLiteModuleConfigRepository{db: db}
}

func (repo *SQLiteModuleConfigRepository) Get(ctx context.Context) (library.ModuleConfig, error) {
	cfg := library.DefaultModuleConfig()
	row := new(moduleConfigRow)
	if err := repo.db.NewSelect().Model(row).Where("id = ?", moduleConfigRowID).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return cfg, nil
		}
		return library.ModuleConfig{}, err
	}
	if row.ConfigJSON == "" {
		return cfg, nil
	}
	if err := json.Unmarshal([]byte(row.ConfigJSON), &cfg); err != nil {
		return library.ModuleConfig{}, err
	}
	return library.NormalizeModuleConfig(cfg), nil
}

func (repo *SQLiteModuleConfigRepository) Save(ctx context.Context, config library.ModuleConfig) error {
	configJSON, err := json.Marshal(library.NormalizeModuleConfig(config))
	if err != nil {
		return err
	}
	row := moduleConfigRow{
		ID:         moduleConfigRowID,
		ConfigJSON: string(configJSON),
		UpdatedAt:  time.Now().UTC(),
	}
	_, err = repo.db.NewInsert().Model(&row).
		On("CONFLICT(id) DO UPDATE").
		Set("config_json = EXCLUDED.config_json").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}
