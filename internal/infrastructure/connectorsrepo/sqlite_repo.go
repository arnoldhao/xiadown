package connectorsrepo

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"xiadown/internal/infrastructure/persistence/sqlitedto"

	"github.com/uptrace/bun"

	"xiadown/internal/domain/connectors"
)

type SQLiteConnectorRepository struct {
	db *bun.DB
}

type connectorRow = sqlitedto.ConnectorRow

func NewSQLiteConnectorRepository(db *bun.DB) *SQLiteConnectorRepository {
	return &SQLiteConnectorRepository{db: db}
}

func (repo *SQLiteConnectorRepository) List(ctx context.Context) ([]connectors.Connector, error) {
	rows := make([]connectorRow, 0)
	if err := repo.db.NewSelect().Model(&rows).Order("created_at ASC").Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]connectors.Connector, 0, len(rows))
	for _, row := range rows {
		connector, err := connectors.NewConnector(connectors.ConnectorParams{
			ID:             row.ID,
			Type:           row.Type,
			Status:         row.Status,
			CredentialMode: stringOrEmpty(row.CredentialMode),
			CookiesPath:    stringOrEmpty(row.CookiesPath),
			CookiesJSON:    stringOrEmpty(row.CookiesJSON),
			ProfileKey:     stringOrEmpty(row.ProfileKey),
			ProfilePath:    stringOrEmpty(row.ProfilePath),
			ProfileBrowser: stringOrEmpty(row.ProfileBrowser),
			LastVerifiedAt: timeOrNil(row.LastVerifiedAt),
			CreatedAt:      &row.CreatedAt,
			UpdatedAt:      &row.UpdatedAt,
		})
		if err != nil {
			return nil, err
		}
		result = append(result, connector)
	}
	return result, nil
}

func (repo *SQLiteConnectorRepository) Get(ctx context.Context, id string) (connectors.Connector, error) {
	row := new(connectorRow)
	if err := repo.db.NewSelect().Model(row).Where("id = ?", id).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return connectors.Connector{}, connectors.ErrConnectorNotFound
		}
		return connectors.Connector{}, err
	}
	return connectors.NewConnector(connectors.ConnectorParams{
		ID:             row.ID,
		Type:           row.Type,
		Status:         row.Status,
		CredentialMode: stringOrEmpty(row.CredentialMode),
		CookiesPath:    stringOrEmpty(row.CookiesPath),
		CookiesJSON:    stringOrEmpty(row.CookiesJSON),
		ProfileKey:     stringOrEmpty(row.ProfileKey),
		ProfilePath:    stringOrEmpty(row.ProfilePath),
		ProfileBrowser: stringOrEmpty(row.ProfileBrowser),
		LastVerifiedAt: timeOrNil(row.LastVerifiedAt),
		CreatedAt:      &row.CreatedAt,
		UpdatedAt:      &row.UpdatedAt,
	})
}

func (repo *SQLiteConnectorRepository) Save(ctx context.Context, connector connectors.Connector) error {
	createdAt := connector.CreatedAt
	updatedAt := connector.UpdatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}
	row := connectorRow{
		ID:             connector.ID,
		Type:           string(connector.Type),
		Status:         string(connector.Status),
		CredentialMode: nullString(string(connector.CredentialMode)),
		CookiesPath:    nullString(connector.CookiesPath),
		CookiesJSON:    nullString(connector.CookiesJSON),
		ProfileKey:     nullString(connector.ProfileKey),
		ProfilePath:    nullString(connector.ProfilePath),
		ProfileBrowser: nullString(connector.ProfileBrowser),
		LastVerifiedAt: nullTime(connector.LastVerifiedAt),
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}
	_, err := repo.db.NewInsert().Model(&row).
		On("CONFLICT(id) DO UPDATE").
		Set("type = EXCLUDED.type").
		Set("status = EXCLUDED.status").
		Set("credential_mode = EXCLUDED.credential_mode").
		Set("cookies_path = EXCLUDED.cookies_path").
		Set("cookies_json = EXCLUDED.cookies_json").
		Set("profile_key = EXCLUDED.profile_key").
		Set("profile_path = EXCLUDED.profile_path").
		Set("profile_browser = EXCLUDED.profile_browser").
		Set("last_verified_at = EXCLUDED.last_verified_at").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func (repo *SQLiteConnectorRepository) Delete(ctx context.Context, id string) error {
	_, err := repo.db.NewDelete().Model((*connectorRow)(nil)).Where("id = ?", id).Exec(ctx)
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
