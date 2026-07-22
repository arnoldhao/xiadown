-- Reduced, schema-faithful fixture captured from tag v0.4.7.
--
-- It intentionally contains the legacy tables and columns that participate in
-- the 1.0 catalog and App Session migrations, plus representative user data.
-- Keeping this as SQL (instead of a generated current-schema database) makes
-- accidental changes to the historical contract visible in review.

PRAGMA foreign_keys = ON;

CREATE TABLE library_libraries (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  created_by_json TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL
);

CREATE TABLE library_files (
  id TEXT PRIMARY KEY,
  library_id TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN (
    'video','audio','subtitle','thumbnail','transcode','other','document','font','api','archive','manifest'
  )),
  name TEXT NOT NULL,
  metadata_json TEXT,
  display_name TEXT,

  storage_mode TEXT NOT NULL CHECK (storage_mode IN ('local_path','db_document','hybrid')),
  storage_local_path TEXT,
  storage_document_id TEXT,

  origin_kind TEXT NOT NULL CHECK (origin_kind IN ('import','download','transcode')),
  origin_operation_id TEXT,
  origin_import_batch_id TEXT,
  origin_import_path TEXT,
  origin_imported_at TIMESTAMP,
  origin_keep_source_file BOOLEAN,

  lineage_root_file_id TEXT,
  latest_operation_id TEXT,

  state_json TEXT NOT NULL,
  media_json TEXT,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,

  FOREIGN KEY (library_id) REFERENCES library_libraries(id) ON DELETE CASCADE,
  FOREIGN KEY (lineage_root_file_id) REFERENCES library_files(id) ON DELETE SET NULL,

  CHECK (
    (kind IN ('video','audio','thumbnail','transcode','other','document','font','api','archive','manifest')
      AND storage_mode IN ('local_path','hybrid') AND COALESCE(storage_local_path,'') <> '') OR
    (kind = 'subtitle' AND storage_mode IN ('db_document','hybrid') AND COALESCE(storage_document_id,'') <> '')
  ),
  CHECK (
    (origin_kind = 'import' AND COALESCE(origin_import_path,'') <> '' AND origin_operation_id IS NULL) OR
    (origin_kind IN ('download','transcode') AND COALESCE(origin_operation_id,'') <> '' AND origin_import_path IS NULL)
  )
);

CREATE INDEX library_files_library_created_idx
  ON library_files(library_id, created_at DESC);

CREATE TABLE library_subtitle_documents (
  id TEXT PRIMARY KEY,
  file_id TEXT NOT NULL UNIQUE,
  library_id TEXT NOT NULL,
  format TEXT NOT NULL,
  original_content TEXT NOT NULL,
  working_content TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  FOREIGN KEY (file_id) REFERENCES library_files(id) ON DELETE CASCADE,
  FOREIGN KEY (library_id) REFERENCES library_libraries(id) ON DELETE CASCADE
);

CREATE TABLE site_app_sessions (
  id TEXT PRIMARY KEY,
  site_key TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL,
  account_display_name TEXT,
  account_handle TEXT,
  account_avatar_url TEXT,
  account_tier_key TEXT,
  account_tier_label TEXT,
  account_badges_json TEXT,
  account_metadata_json TEXT,
  account_verification_status TEXT NOT NULL DEFAULT 'unverified',
  account_verification_error TEXT,
  account_verification_started_at TIMESTAMP,
  last_verified_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO library_libraries (
  id, name, created_by_json, created_at, updated_at
) VALUES
  ('legacy-bundle', 'v0.4.7 Library', '{"source":"fixture"}', '2026-04-01T08:00:00Z', '2026-04-02T08:00:00Z'),
  ('legacy-empty', 'Empty bundle', '{}', '2026-04-01T08:00:00Z', '2026-04-01T08:00:00Z');

INSERT INTO library_files (
  id, library_id, kind, name, metadata_json, display_name,
  storage_mode, storage_local_path, storage_document_id,
  origin_kind, origin_operation_id, origin_import_batch_id, origin_import_path,
  origin_imported_at, origin_keep_source_file, lineage_root_file_id, latest_operation_id,
  state_json, media_json, created_at, updated_at
) VALUES (
  'legacy-video', 'legacy-bundle', 'video', 'movie.mp4',
  '{"Title":"Fixture Movie","Author":"Fixture Director"}', 'Fixture Movie',
  'local_path', '/fixture/media/movie.mp4', NULL,
  'import', NULL, 'legacy-import', '/fixture/media/movie.mp4',
  '2026-04-01T08:00:00Z', 0, NULL, NULL,
  '{"Status":"active","Deleted":false}',
  '{"Format":"mp4","VideoCodec":"h264","DurationMs":90000,"Width":1920,"Height":1080,"SizeBytes":42000000}',
  '2026-04-01T08:00:00Z', '2026-04-02T08:00:00Z'
);

INSERT INTO library_files (
  id, library_id, kind, name, metadata_json, display_name,
  storage_mode, storage_local_path, storage_document_id,
  origin_kind, origin_operation_id, origin_import_batch_id, origin_import_path,
  origin_imported_at, origin_keep_source_file, lineage_root_file_id, latest_operation_id,
  state_json, media_json, created_at, updated_at
) VALUES (
  'legacy-subtitle', 'legacy-bundle', 'subtitle', 'movie.zh-CN.srt',
  '{"Title":"Fixture subtitle"}', 'Fixture subtitle',
  'db_document', NULL, 'legacy-subtitle-document',
  'import', NULL, 'legacy-import', '/fixture/media/movie.zh-CN.srt',
  '2026-04-01T08:00:00Z', 1, 'legacy-video', NULL,
  '{"Status":"active","Deleted":false}', NULL,
  '2026-04-01T08:01:00Z', '2026-04-02T08:01:00Z'
);

INSERT INTO library_subtitle_documents (
  id, file_id, library_id, format, original_content, working_content, created_at, updated_at
) VALUES (
  'legacy-subtitle-document', 'legacy-subtitle', 'legacy-bundle', 'srt',
  '1\n00:00:00,000 --> 00:00:01,000\nLegacy subtitle',
  '1\n00:00:00,000 --> 00:00:01,000\nLegacy subtitle',
  '2026-04-01T08:01:00Z', '2026-04-02T08:01:00Z'
);

INSERT INTO site_app_sessions (
  id, site_key, status, account_display_name, account_handle,
  account_badges_json, account_metadata_json, account_verification_status,
  created_at, updated_at
) VALUES (
  'legacy-youtube-session', 'youtube', 'connected', 'Legacy User', '@legacy',
  '["legacy"]', '{"fixture":true}', 'verified',
  '2026-04-01T09:00:00Z', '2026-04-02T09:00:00Z'
);
