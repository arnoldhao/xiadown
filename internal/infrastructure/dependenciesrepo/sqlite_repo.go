package dependenciesrepo

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"xiadown/internal/infrastructure/persistence/sqlitedto"

	"github.com/uptrace/bun"

	"xiadown/internal/domain/dependencies"
)

type SQLiteDependencyRepository struct {
	db *bun.DB
}

type dependencyRow = sqlitedto.DependencyRow

func NewSQLiteDependencyRepository(db *bun.DB) *SQLiteDependencyRepository {
	return &SQLiteDependencyRepository{db: db}
}

func (repo *SQLiteDependencyRepository) List(ctx context.Context) ([]dependencies.Dependency, error) {
	rows := make([]dependencyRow, 0)
	if err := repo.db.NewSelect().Model(&rows).Order("name ASC").Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]dependencies.Dependency, 0, len(rows))
	for _, row := range rows {
		item, err := dependencies.NewDependency(dependencies.DependencyParams{
			Name:        row.Name,
			ExecPath:    stringOrEmpty(row.ExecPath),
			Version:     stringOrEmpty(row.Version),
			Status:      stringOrEmpty(row.Status),
			InstalledAt: timeOrNil(row.InstalledAt),
			UpdatedAt:   &row.UpdatedAt,
		})
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (repo *SQLiteDependencyRepository) Get(ctx context.Context, name string) (dependencies.Dependency, error) {
	row := new(dependencyRow)
	if err := repo.db.NewSelect().Model(row).Where("name = ?", name).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return dependencies.Dependency{}, dependencies.ErrDependencyNotFound
		}
		return dependencies.Dependency{}, err
	}
	return dependencies.NewDependency(dependencies.DependencyParams{
		Name:        row.Name,
		ExecPath:    stringOrEmpty(row.ExecPath),
		Version:     stringOrEmpty(row.Version),
		Status:      stringOrEmpty(row.Status),
		InstalledAt: timeOrNil(row.InstalledAt),
		UpdatedAt:   &row.UpdatedAt,
	})
}

func (repo *SQLiteDependencyRepository) Save(ctx context.Context, tool dependencies.Dependency) error {
	updatedAt := tool.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}
	row := dependencyRow{
		Name:        string(tool.Name),
		ExecPath:    nullString(tool.ExecPath),
		Version:     nullString(tool.Version),
		Status:      nullString(string(tool.Status)),
		InstalledAt: nullTime(tool.InstalledAt),
		UpdatedAt:   updatedAt,
	}
	_, err := repo.db.NewInsert().Model(&row).
		On("CONFLICT(name) DO UPDATE").
		Set("exec_path = EXCLUDED.exec_path").
		Set("version = EXCLUDED.version").
		Set("status = EXCLUDED.status").
		Set("installed_at = EXCLUDED.installed_at").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func (repo *SQLiteDependencyRepository) Delete(ctx context.Context, name string) error {
	_, err := repo.db.NewDelete().Model((*dependencyRow)(nil)).Where("name = ?", name).Exec(ctx)
	return err
}

func nullString(value string) sql.NullString {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: trimmed, Valid: true}
}

func stringOrEmpty(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func nullTime(value *time.Time) sql.NullTime {
	if value == nil || value.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *value, Valid: true}
}

func timeOrNil(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}
