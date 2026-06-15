package appsessionsrepo

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/uptrace/bun"

	"xiadown/internal/domain/appsessions"
	"xiadown/internal/infrastructure/persistence/sqlitedto"
)

type SQLiteRepository struct {
	db *bun.DB
}

type siteAppSessionRow = sqlitedto.SiteAppSessionRow

func NewSQLiteRepository(db *bun.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (repo *SQLiteRepository) List(ctx context.Context) ([]appsessions.Session, error) {
	rows := []siteAppSessionRow{}
	if err := repo.db.NewSelect().Model(&rows).Order("site_key ASC").Scan(ctx); err != nil {
		return nil, err
	}
	result := make([]appsessions.Session, 0, len(rows))
	for _, row := range rows {
		session, err := rowToSession(row)
		if err != nil {
			return nil, err
		}
		result = append(result, session)
	}
	return result, nil
}

func (repo *SQLiteRepository) Get(ctx context.Context, id string) (appsessions.Session, error) {
	row := new(siteAppSessionRow)
	if err := repo.db.NewSelect().Model(row).Where("id = ?", strings.TrimSpace(id)).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appsessions.Session{}, appsessions.ErrSessionNotFound
		}
		return appsessions.Session{}, err
	}
	return rowToSession(*row)
}

func (repo *SQLiteRepository) GetBySiteKey(ctx context.Context, siteKey string) (appsessions.Session, error) {
	row := new(siteAppSessionRow)
	if err := repo.db.NewSelect().Model(row).Where("site_key = ?", strings.TrimSpace(siteKey)).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appsessions.Session{}, appsessions.ErrSessionNotFound
		}
		return appsessions.Session{}, err
	}
	return rowToSession(*row)
}

func (repo *SQLiteRepository) Save(ctx context.Context, session appsessions.Session) error {
	createdAt := session.CreatedAt
	updatedAt := session.UpdatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}
	row := siteAppSessionRow{
		ID:                           session.ID,
		SiteKey:                      session.SiteKey,
		Status:                       string(session.Status),
		AccountDisplayName:           nullString(session.AccountDisplayName),
		AccountHandle:                nullString(session.AccountHandle),
		AccountAvatarURL:             nullString(session.AccountAvatarURL),
		AccountTierKey:               nullString(session.AccountTierKey),
		AccountTierLabel:             nullString(session.AccountTierLabel),
		AccountBadgesJSON:            nullString(session.AccountBadgesJSON),
		AccountMetadataJSON:          nullString(session.AccountMetadataJSON),
		AccountVerificationStatus:    nullString(string(session.AccountVerificationStatus)),
		AccountVerificationError:     nullString(session.AccountVerificationError),
		AccountVerificationStartedAt: nullTime(session.AccountVerificationStartedAt),
		LastVerifiedAt:               nullTime(session.LastVerifiedAt),
		CreatedAt:                    createdAt,
		UpdatedAt:                    updatedAt,
	}
	_, err := repo.db.NewInsert().Model(&row).
		On("CONFLICT(id) DO UPDATE").
		Set("site_key = EXCLUDED.site_key").
		Set("status = EXCLUDED.status").
		Set("account_display_name = EXCLUDED.account_display_name").
		Set("account_handle = EXCLUDED.account_handle").
		Set("account_avatar_url = EXCLUDED.account_avatar_url").
		Set("account_tier_key = EXCLUDED.account_tier_key").
		Set("account_tier_label = EXCLUDED.account_tier_label").
		Set("account_badges_json = EXCLUDED.account_badges_json").
		Set("account_metadata_json = EXCLUDED.account_metadata_json").
		Set("account_verification_status = EXCLUDED.account_verification_status").
		Set("account_verification_error = EXCLUDED.account_verification_error").
		Set("account_verification_started_at = EXCLUDED.account_verification_started_at").
		Set("last_verified_at = EXCLUDED.last_verified_at").
		Set("updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

func (repo *SQLiteRepository) Delete(ctx context.Context, id string) error {
	_, err := repo.db.NewDelete().Model((*siteAppSessionRow)(nil)).Where("id = ?", strings.TrimSpace(id)).Exec(ctx)
	return err
}

func rowToSession(row siteAppSessionRow) (appsessions.Session, error) {
	return appsessions.NewSession(appsessions.SessionParams{
		ID:                           row.ID,
		SiteKey:                      row.SiteKey,
		Status:                       row.Status,
		AccountDisplayName:           stringOrEmpty(row.AccountDisplayName),
		AccountHandle:                stringOrEmpty(row.AccountHandle),
		AccountAvatarURL:             stringOrEmpty(row.AccountAvatarURL),
		AccountTierKey:               stringOrEmpty(row.AccountTierKey),
		AccountTierLabel:             stringOrEmpty(row.AccountTierLabel),
		AccountBadgesJSON:            stringOrEmpty(row.AccountBadgesJSON),
		AccountMetadataJSON:          stringOrEmpty(row.AccountMetadataJSON),
		AccountVerificationStatus:    stringOrEmpty(row.AccountVerificationStatus),
		AccountVerificationError:     stringOrEmpty(row.AccountVerificationError),
		AccountVerificationStartedAt: timeOrNil(row.AccountVerificationStartedAt),
		LastVerifiedAt:               timeOrNil(row.LastVerifiedAt),
		CreatedAt:                    &row.CreatedAt,
		UpdatedAt:                    &row.UpdatedAt,
	})
}

func nullString(value string) sql.NullString {
	trimmed := strings.TrimSpace(value)
	return sql.NullString{String: trimmed, Valid: trimmed != ""}
}

func stringOrEmpty(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
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
