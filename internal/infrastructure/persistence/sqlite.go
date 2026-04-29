package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/extra/bundebug"
)

type SQLiteConfig struct {
	Path string
}

type Database struct {
	SQL *sql.DB
	Bun *bun.DB
}

func (db *Database) Close() error {
	if db == nil || db.SQL == nil {
		return nil
	}
	return db.SQL.Close()
}

func OpenSQLite(ctx context.Context, config SQLiteConfig) (*Database, error) {
	if config.Path == "" {
		return nil, fmt.Errorf("sqlite path is required")
	}

	sqlDB, err := sql.Open("sqlite3", config.Path)
	if err != nil {
		return nil, err
	}

	if err := applyPragmas(ctx, sqlDB); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	if err := applySchema(ctx, sqlDB); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}

	bunDB := bun.NewDB(sqlDB, sqlitedialect.New())
	if os.Getenv("XIADOWN_SQL_DEBUG") != "" {
		bunDB.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(true)))
	}

	return &Database{SQL: sqlDB, Bun: bunDB}, nil
}

func applyPragmas(ctx context.Context, db *sql.DB) error {
	statements := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func applySchema(ctx context.Context, db *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS settings (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	appearance TEXT NOT NULL CHECK (appearance IN ('light','dark','auto')),
	font_family TEXT,
	theme_color TEXT,
	color_scheme TEXT,
	font_size INTEGER,
	language TEXT,
	download_directory TEXT,
	log_level TEXT,
	log_max_size_mb INTEGER,
	log_max_backups INTEGER,
	log_max_age_days INTEGER,
	log_compress BOOLEAN,
	menu_bar_visibility TEXT DEFAULT 'whenRunning',
	auto_start BOOLEAN,
	minimize_to_tray_on_start BOOLEAN,
	agent_model_provider_id TEXT,
	agent_model_name TEXT,
	chat_stream_enabled BOOLEAN,
	chat_temperature REAL,
	chat_max_tokens INTEGER,
	skills_json TEXT,
	gateway_flags_json TEXT,
	memory_json TEXT,
	appearance_config_json TEXT,
	tools_config_json TEXT,
	skills_config_json TEXT,
	main_x INTEGER,
	main_y INTEGER,
	main_width INTEGER,
	main_height INTEGER,
	settings_x INTEGER,
	settings_y INTEGER,
	settings_width INTEGER,
	settings_height INTEGER,
	proxy_mode TEXT,
	proxy_scheme TEXT,
	proxy_host TEXT,
	proxy_port INTEGER,
	proxy_username TEXT,
	proxy_password TEXT,
	proxy_no_proxy TEXT,
	proxy_timeout_seconds INTEGER,
	proxy_tested_at TIMESTAMP,
	proxy_test_success BOOLEAN,
	proxy_test_message TEXT,
	commands_json TEXT,
	channels_json TEXT,
	version INTEGER NOT NULL DEFAULT 1,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER IF NOT EXISTS settings_updated_at
AFTER UPDATE ON settings
FOR EACH ROW
BEGIN
	UPDATE settings SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TABLE IF NOT EXISTS config_revisions (
	id TEXT PRIMARY KEY,
	version TEXT NOT NULL,
	payload_json TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS diagnostic_reports (
	id TEXT PRIMARY KEY,
	payload_json TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS telemetry_state (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	install_id TEXT NOT NULL,
	install_created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	launch_count INTEGER NOT NULL DEFAULT 0,
	distinct_days_used INTEGER NOT NULL DEFAULT 0,
	distinct_days_used_last_month INTEGER NOT NULL DEFAULT 0,
	completed_session_count INTEGER NOT NULL DEFAULT 0,
	total_session_seconds REAL NOT NULL DEFAULT 0,
	previous_session_seconds REAL,
	first_chat_completed_at TIMESTAMP,
	first_library_completed_at TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TRIGGER IF NOT EXISTS telemetry_state_updated_at
AFTER UPDATE ON telemetry_state
FOR EACH ROW
BEGIN
	UPDATE telemetry_state SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TABLE IF NOT EXISTS telemetry_session_days (
	day TEXT PRIMARY KEY,
	first_seen_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS llm_call_records (
	id TEXT PRIMARY KEY,
	session_id TEXT,
	thread_id TEXT,
	run_id TEXT,
	provider_id TEXT,
	model_name TEXT,
	request_source TEXT,
	operation TEXT,
	status TEXT NOT NULL,
	finish_reason TEXT,
	error_text TEXT,
	input_tokens INTEGER,
	output_tokens INTEGER,
	total_tokens INTEGER,
	context_prompt_tokens INTEGER,
	context_total_tokens INTEGER,
	context_window_tokens INTEGER,
	request_payload_json TEXT,
	response_payload_json TEXT,
	payload_truncated BOOLEAN NOT NULL DEFAULT 0,
	started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	finished_at TIMESTAMP,
	duration_ms INTEGER
);

CREATE INDEX IF NOT EXISTS llm_call_records_started_at_idx
	ON llm_call_records(started_at DESC);
CREATE INDEX IF NOT EXISTS llm_call_records_thread_started_at_idx
	ON llm_call_records(thread_id, started_at DESC);
CREATE INDEX IF NOT EXISTS llm_call_records_run_started_at_idx
	ON llm_call_records(run_id, started_at DESC);
CREATE INDEX IF NOT EXISTS llm_call_records_model_started_at_idx
	ON llm_call_records(provider_id, model_name, started_at DESC);

CREATE TABLE IF NOT EXISTS usage_ledger (
	id TEXT PRIMARY KEY,
	category TEXT,
	provider_id TEXT,
	model_name TEXT,
	channel TEXT,
	request_id TEXT,
	request_source TEXT,
	units INTEGER,
	prompt_tokens INTEGER,
	completion_tokens INTEGER,
	cost_micros INTEGER,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS usage_ledger_created_at_idx ON usage_ledger(created_at DESC);

CREATE TABLE IF NOT EXISTS usage_events (
	id TEXT PRIMARY KEY,
	request_id TEXT NOT NULL,
	step_id TEXT NOT NULL,
	provider_id TEXT NOT NULL,
	model_name TEXT NOT NULL,
	category TEXT,
	channel TEXT,
	request_source TEXT NOT NULL,
	usage_status TEXT,
	input_tokens INTEGER,
	output_tokens INTEGER,
	total_tokens INTEGER,
	cached_input_tokens INTEGER,
	reasoning_tokens INTEGER,
	audio_input_tokens INTEGER,
	audio_output_tokens INTEGER,
	raw_usage_json TEXT,
	occurred_at TIMESTAMP NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(request_id, step_id, provider_id, model_name)
);

CREATE INDEX IF NOT EXISTS usage_events_occurred_idx
	ON usage_events(occurred_at DESC, provider_id, model_name, request_source);

CREATE TABLE IF NOT EXISTS model_pricing_versions (
	id TEXT PRIMARY KEY,
	provider_id TEXT NOT NULL,
	model_name TEXT NOT NULL,
	currency TEXT NOT NULL DEFAULT 'USD',
	input_per_million REAL NOT NULL DEFAULT 0,
	output_per_million REAL NOT NULL DEFAULT 0,
	cached_input_per_million REAL NOT NULL DEFAULT 0,
	reasoning_per_million REAL NOT NULL DEFAULT 0,
	audio_input_per_million REAL NOT NULL DEFAULT 0,
	audio_output_per_million REAL NOT NULL DEFAULT 0,
	per_request REAL NOT NULL DEFAULT 0,
	source TEXT NOT NULL DEFAULT 'manual',
	effective_from TIMESTAMP NOT NULL,
	effective_to TIMESTAMP,
	is_active BOOLEAN NOT NULL DEFAULT 1,
	updated_by TEXT,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(provider_id, model_name, effective_from, source)
);

CREATE INDEX IF NOT EXISTS model_pricing_versions_lookup_idx
	ON model_pricing_versions(provider_id, model_name, effective_from DESC, is_active);

CREATE TABLE IF NOT EXISTS usage_ledger_entries (
	id TEXT PRIMARY KEY,
	event_id TEXT NOT NULL,
	request_id TEXT NOT NULL,
	category TEXT NOT NULL DEFAULT 'tokens',
	provider_id TEXT NOT NULL,
	model_name TEXT NOT NULL,
	channel TEXT,
	request_source TEXT NOT NULL,
	cost_basis TEXT NOT NULL DEFAULT 'estimated',
	pricing_version_id TEXT NOT NULL DEFAULT '',
	units INTEGER,
	input_tokens INTEGER,
	output_tokens INTEGER,
	cached_input_tokens INTEGER,
	reasoning_tokens INTEGER,
	input_cost_micros INTEGER,
	output_cost_micros INTEGER,
	cached_input_cost_micros INTEGER,
	reasoning_cost_micros INTEGER,
	request_cost_micros INTEGER,
	total_cost_micros INTEGER,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(event_id, pricing_version_id, cost_basis, category)
);

CREATE INDEX IF NOT EXISTS usage_ledger_entries_created_idx
	ON usage_ledger_entries(created_at DESC, provider_id, model_name, request_source);

CREATE TABLE IF NOT EXISTS tts_jobs (
	id TEXT PRIMARY KEY,
	provider_id TEXT,
	voice_id TEXT,
	model_id TEXT,
	format TEXT,
	status TEXT,
	input_text TEXT,
	output_json TEXT,
	cost_micros INTEGER,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS voicewake_config (
	id TEXT PRIMARY KEY,
	version INTEGER NOT NULL DEFAULT 1,
	triggers_json TEXT,
	tts_config_json TEXT,
	talk_config_json TEXT,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS threads (
	id TEXT PRIMARY KEY,
	agent_id TEXT,
	assistant_id TEXT,
	title TEXT,
	title_is_default BOOLEAN NOT NULL DEFAULT 0,
	title_changed_by TEXT,
	status TEXT NOT NULL DEFAULT 'regular',
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	last_interactive_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMP,
	purge_after TIMESTAMP
);

CREATE INDEX IF NOT EXISTS threads_agent_id ON threads(agent_id);
CREATE INDEX IF NOT EXISTS threads_assistant_id ON threads(assistant_id);

CREATE TABLE IF NOT EXISTS assistants (
	id TEXT PRIMARY KEY,
	identity_json TEXT,
	avatar_json TEXT,
	user_json TEXT,
	model_json TEXT,
	tools_json TEXT,
	skills_json TEXT,
	call_json TEXT,
	memory_json TEXT,
	builtin BOOLEAN NOT NULL DEFAULT 0,
	deletable BOOLEAN NOT NULL DEFAULT 1,
	enabled BOOLEAN NOT NULL DEFAULT 1,
	is_default BOOLEAN NOT NULL DEFAULT 0,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS assistants_enabled ON assistants(enabled);
CREATE INDEX IF NOT EXISTS assistants_is_default ON assistants(is_default);

CREATE TABLE IF NOT EXISTS agents (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	description TEXT,
	enabled BOOLEAN NOT NULL DEFAULT 1,
	thread_id TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMP,
	FOREIGN KEY (thread_id) REFERENCES threads(id)
);

CREATE INDEX IF NOT EXISTS agents_enabled ON agents(enabled);

CREATE TABLE IF NOT EXISTS thread_messages (
	id TEXT PRIMARY KEY,
	thread_id TEXT NOT NULL,
	kind TEXT NOT NULL DEFAULT 'chat',
	role TEXT NOT NULL,
	content TEXT NOT NULL,
	parts_json TEXT NOT NULL DEFAULT '[]',
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS thread_runs (
	id TEXT PRIMARY KEY,
	thread_id TEXT NOT NULL,
	assistant_message_id TEXT NOT NULL,
	user_message_id TEXT,
	agent_id TEXT,
	status TEXT NOT NULL,
	content_partial TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS thread_runs_thread_id ON thread_runs(thread_id);
CREATE INDEX IF NOT EXISTS thread_runs_agent_id ON thread_runs(agent_id);
CREATE INDEX IF NOT EXISTS thread_runs_status ON thread_runs(status);

CREATE TABLE IF NOT EXISTS agent_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	run_id TEXT NOT NULL,
	thread_id TEXT NOT NULL,
	event_name TEXT NOT NULL,
	payload_json TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (run_id) REFERENCES thread_runs(id) ON DELETE CASCADE,
	FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS agent_events_run_id ON agent_events(run_id, id ASC);
CREATE INDEX IF NOT EXISTS agent_events_thread_id ON agent_events(thread_id, id ASC);

CREATE TABLE IF NOT EXISTS memory_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	assistant_id TEXT NOT NULL,
	thread_id TEXT,
	run_id TEXT,
	event_type TEXT NOT NULL,
	payload_json TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS memory_events_assistant_id ON memory_events(assistant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS memory_events_thread_id ON memory_events(thread_id, created_at DESC);

CREATE TABLE IF NOT EXISTS memory_profiles (
	assistant_id TEXT NOT NULL,
	profile_key TEXT NOT NULL,
	profile_value TEXT NOT NULL,
	confidence REAL NOT NULL DEFAULT 0,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (assistant_id, profile_key)
);

CREATE INDEX IF NOT EXISTS memory_profiles_assistant_id ON memory_profiles(assistant_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS memory_collections (
	id TEXT PRIMARY KEY,
	assistant_id TEXT NOT NULL,
	thread_id TEXT,
	category TEXT NOT NULL DEFAULT '',
	content TEXT NOT NULL,
	metadata_json TEXT NOT NULL DEFAULT '{}',
	confidence REAL NOT NULL DEFAULT 0,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS memory_collections_assistant_id ON memory_collections(assistant_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS memory_collections_thread_id ON memory_collections(thread_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS memory_files (
	assistant_id TEXT NOT NULL,
	file_path TEXT NOT NULL,
	content TEXT NOT NULL,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (assistant_id, file_path)
);

CREATE TABLE IF NOT EXISTS memory_chunks (
	chunk_id TEXT PRIMARY KEY,
	assistant_id TEXT NOT NULL,
	thread_id TEXT,
	file_path TEXT NOT NULL,
	line_start INTEGER NOT NULL,
	line_end INTEGER NOT NULL,
	content TEXT NOT NULL,
	embedding_json TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS memory_chunks_assistant_file ON memory_chunks(assistant_id, file_path, line_start ASC);

CREATE TABLE IF NOT EXISTS tool_runs (
	id TEXT PRIMARY KEY,
	run_id TEXT NOT NULL,
	tool_call_id TEXT,
	tool_name TEXT NOT NULL,
	input_hash TEXT NOT NULL,
	input_json TEXT,
	output_json TEXT,
	error_text TEXT,
	job_id TEXT,
	status TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	started_at TIMESTAMP,
	finished_at TIMESTAMP,
	FOREIGN KEY (run_id) REFERENCES thread_runs(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS tool_runs_idempotency ON tool_runs(run_id, tool_name, input_hash);
CREATE INDEX IF NOT EXISTS tool_runs_run_id ON tool_runs(run_id);

CREATE TABLE IF NOT EXISTS tool_policy_audit (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	tool_id TEXT,
	decision TEXT NOT NULL,
	reason TEXT,
	context_json TEXT,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS exec_approvals (
	id TEXT PRIMARY KEY,
	request_json TEXT NOT NULL,
	status TEXT NOT NULL,
	decision TEXT,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	resolved_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS skills_install_jobs (
	id TEXT PRIMARY KEY,
	provider_id TEXT,
	status TEXT NOT NULL,
	payload_json TEXT,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS automation_jobs (
	id TEXT PRIMARY KEY,
	kind TEXT NOT NULL,
	status TEXT NOT NULL,
	config_json TEXT,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS automation_runs (
	id TEXT PRIMARY KEY,
	job_id TEXT NOT NULL,
	status TEXT NOT NULL,
	error TEXT,
	started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	ended_at TIMESTAMP,
	FOREIGN KEY (job_id) REFERENCES automation_jobs(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS automation_trigger_logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	job_id TEXT,
	event_id TEXT,
	payload_json TEXT,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS cron_jobs (
	id TEXT PRIMARY KEY,
	assistant_id TEXT NOT NULL,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	enabled BOOLEAN NOT NULL DEFAULT 1,
	delete_after_run BOOLEAN NOT NULL DEFAULT 0,
	schedule_json TEXT NOT NULL,
	payload_json TEXT NOT NULL,
	delivery_json TEXT NOT NULL DEFAULT '',
	session_target TEXT NOT NULL,
	wake_mode TEXT NOT NULL,
	session_key TEXT NOT NULL DEFAULT '',
	state_json TEXT NOT NULL DEFAULT '',
	created_at_ms INTEGER NOT NULL,
	updated_at_ms INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS cron_jobs_updated_at_ms ON cron_jobs(updated_at_ms DESC);
CREATE INDEX IF NOT EXISTS cron_jobs_enabled ON cron_jobs(enabled, updated_at_ms DESC);
CREATE INDEX IF NOT EXISTS cron_jobs_assistant ON cron_jobs(assistant_id, updated_at_ms DESC);

CREATE TABLE IF NOT EXISTS cron_runs (
	run_id TEXT PRIMARY KEY,
	job_id TEXT NOT NULL,
	status TEXT NOT NULL,
	error TEXT NOT NULL DEFAULT '',
	summary TEXT NOT NULL DEFAULT '',
	delivery_status TEXT NOT NULL DEFAULT '',
	delivery_error TEXT NOT NULL DEFAULT '',
	session_key TEXT NOT NULL DEFAULT '',
	model TEXT NOT NULL DEFAULT '',
	provider TEXT NOT NULL DEFAULT '',
	usage_json TEXT NOT NULL DEFAULT '',
	run_at_ms INTEGER NOT NULL,
	duration_ms INTEGER NOT NULL DEFAULT 0,
	created_at_ms INTEGER NOT NULL,
	ended_at_ms INTEGER NOT NULL DEFAULT 0,
	FOREIGN KEY (job_id) REFERENCES cron_jobs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS cron_runs_job_id_run_at_ms ON cron_runs(job_id, run_at_ms DESC);
CREATE INDEX IF NOT EXISTS cron_runs_status_run_at_ms ON cron_runs(status, run_at_ms DESC);
CREATE INDEX IF NOT EXISTS cron_runs_delivery_status ON cron_runs(delivery_status, run_at_ms DESC);

CREATE TABLE IF NOT EXISTS cron_run_events (
	event_id TEXT PRIMARY KEY,
	run_id TEXT NOT NULL,
	job_id TEXT NOT NULL,
	job_name TEXT NOT NULL DEFAULT '',
	stage TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT '',
	message TEXT NOT NULL DEFAULT '',
	error TEXT NOT NULL DEFAULT '',
	channel TEXT NOT NULL DEFAULT '',
	session_key TEXT NOT NULL DEFAULT '',
	source TEXT NOT NULL DEFAULT '',
	meta_json TEXT NOT NULL DEFAULT '',
	created_at_ms INTEGER NOT NULL,
	FOREIGN KEY (run_id) REFERENCES cron_runs(run_id) ON DELETE CASCADE,
	FOREIGN KEY (job_id) REFERENCES cron_jobs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS cron_run_events_run_id_created_at_ms ON cron_run_events(run_id, created_at_ms DESC);
CREATE INDEX IF NOT EXISTS cron_run_events_job_id_created_at_ms ON cron_run_events(job_id, created_at_ms DESC);

CREATE TABLE IF NOT EXISTS connectors (
	id TEXT PRIMARY KEY,
	type TEXT NOT NULL,
	status TEXT NOT NULL,
	cookies_path TEXT,
	cookies_json TEXT,
	last_verified_at TIMESTAMP,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS dependencies (
	name TEXT PRIMARY KEY,
	exec_path TEXT,
	version TEXT,
	status TEXT,
	installed_at TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sprites (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	frame_count INTEGER NOT NULL,
	frame_width INTEGER NOT NULL,
	frame_height INTEGER NOT NULL,
	columns INTEGER NOT NULL,
	rows INTEGER NOT NULL,
	sprite_file TEXT NOT NULL,
	sprite_path TEXT NOT NULL,
	source_type TEXT NOT NULL DEFAULT '',
	origin TEXT NOT NULL DEFAULT '',
	scope TEXT NOT NULL,
	status TEXT NOT NULL,
	validation_message TEXT,
	image_width INTEGER NOT NULL,
	image_height INTEGER NOT NULL,
	author_id TEXT NOT NULL,
	author_display_name TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL,
	version TEXT NOT NULL,
	cover_png BLOB,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS sprites_scope_name_idx
	ON sprites(scope, status, name);

CREATE TABLE IF NOT EXISTS transcode_presets (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	output_type TEXT NOT NULL,
	container TEXT NOT NULL,
	video_codec TEXT,
	audio_codec TEXT,
	quality_mode TEXT,
	crf INTEGER,
	bitrate_kbps INTEGER,
	audio_bitrate_kbps INTEGER,
	scale TEXT,
	width INTEGER,
	height INTEGER,
	ffmpeg_preset TEXT,
	allow_upscale BOOLEAN NOT NULL DEFAULT 0,
	requires_video BOOLEAN NOT NULL DEFAULT 0,
	requires_audio BOOLEAN NOT NULL DEFAULT 0,
	is_builtin BOOLEAN NOT NULL DEFAULT 0,
	description TEXT,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS providers (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	type TEXT NOT NULL,
	compatibility TEXT NOT NULL DEFAULT '',
	endpoint TEXT,
	enabled BOOLEAN NOT NULL DEFAULT 0,
	is_builtin BOOLEAN NOT NULL DEFAULT 0,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS provider_secrets (
	id TEXT PRIMARY KEY,
	provider_id TEXT NOT NULL,
	key_ref TEXT,
	org_ref TEXT,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS provider_models (
	id TEXT PRIMARY KEY,
	provider_id TEXT NOT NULL,
	name TEXT NOT NULL,
	display_name TEXT,
	capabilities_json TEXT,
	context_window_tokens INTEGER,
	max_output_tokens INTEGER,
	supports_tools BOOLEAN,
	supports_reasoning BOOLEAN,
	supports_vision BOOLEAN,
	supports_audio BOOLEAN,
	supports_video BOOLEAN,
	enabled BOOLEAN NOT NULL DEFAULT 0,
	show_in_ui BOOLEAN NOT NULL DEFAULT 0,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS models_dev_catalog (
	id TEXT PRIMARY KEY,
	provider_key TEXT NOT NULL,
	model_name TEXT NOT NULL,
	display_name TEXT,
	capabilities_json TEXT,
	context_window_tokens INTEGER,
	max_output_tokens INTEGER,
	supports_tools BOOLEAN,
	supports_reasoning BOOLEAN,
	supports_vision BOOLEAN,
	supports_audio BOOLEAN,
	supports_video BOOLEAN,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS models_dev_catalog_provider_model_idx ON models_dev_catalog(provider_key, model_name);
CREATE INDEX IF NOT EXISTS models_dev_catalog_model_name_idx ON models_dev_catalog(model_name);

CREATE TABLE IF NOT EXISTS gateway_pair_requests (
	id TEXT PRIMARY KEY,
	node_id TEXT NOT NULL,
	status TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS gateway_device_tokens (
	id TEXT PRIMARY KEY,
	node_id TEXT NOT NULL,
	issued_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	expires_at TIMESTAMP,
	revoked_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS gateway_sessions (
	session_id TEXT PRIMARY KEY,
	session_key TEXT NOT NULL,
	agent_id TEXT,
	assistant_id TEXT,
	title TEXT,
	status TEXT,
	origin_json TEXT,
	context_prompt_tokens INTEGER,
	context_total_tokens INTEGER,
	context_window_tokens INTEGER,
	context_updated_at TIMESTAMP,
	context_fresh BOOLEAN,
	context_summary TEXT,
	context_first_kept_message_id TEXT,
	context_strategy_version INTEGER,
	context_compacted_at TIMESTAMP,
	compaction_count INTEGER,
	memory_flush_compaction_count INTEGER,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS gateway_queue_tickets (
	ticket_id TEXT PRIMARY KEY,
	session_key TEXT NOT NULL,
	lane TEXT,
	status TEXT,
	position INTEGER,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS gateway_events (
	id TEXT PRIMARY KEY,
	event_type TEXT NOT NULL,
	session_id TEXT,
	session_key TEXT,
	payload_json TEXT,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS heartbeat_events (
	id TEXT PRIMARY KEY,
	session_key TEXT,
	thread_id TEXT,
	status TEXT,
	message TEXT,
	error TEXT,
	content_hash TEXT,
	reason TEXT,
	source TEXT,
	run_id TEXT,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_heartbeat_session_created ON heartbeat_events(session_key, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_heartbeat_hash_created ON heartbeat_events(session_key, content_hash, created_at DESC);

CREATE TABLE IF NOT EXISTS notices (
	id TEXT PRIMARY KEY,
	kind TEXT NOT NULL,
	category TEXT NOT NULL,
	code TEXT NOT NULL,
	severity TEXT NOT NULL,
	status TEXT NOT NULL,
	i18n_json TEXT NOT NULL DEFAULT '{}',
	source_json TEXT NOT NULL DEFAULT '{}',
	action_json TEXT NOT NULL DEFAULT '{}',
	surfaces_json TEXT NOT NULL DEFAULT '[]',
	dedup_key TEXT NOT NULL DEFAULT '',
	occurrence_count INTEGER NOT NULL DEFAULT 1,
	metadata_json TEXT NOT NULL DEFAULT '{}',
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	last_occurred_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	read_at TIMESTAMP,
	archived_at TIMESTAMP,
	expires_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_notices_status_last ON notices(status, last_occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_notices_kind_status_last ON notices(kind, status, last_occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_notices_category_status_last ON notices(category, status, last_occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_notices_dedup_key ON notices(dedup_key);

CREATE TABLE IF NOT EXISTS node_registry (
	node_id TEXT PRIMARY KEY,
	display_name TEXT,
	platform TEXT,
	version TEXT,
	capabilities_json TEXT,
	status TEXT,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS node_pair_tokens (
	id TEXT PRIMARY KEY,
	node_id TEXT NOT NULL,
	issued_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	expires_at TIMESTAMP,
	revoked_at TIMESTAMP,
	FOREIGN KEY (node_id) REFERENCES node_registry(node_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS node_invoke_logs (
	id TEXT PRIMARY KEY,
	node_id TEXT NOT NULL,
	capability TEXT NOT NULL,
	action TEXT,
	args_json TEXT,
	status TEXT NOT NULL,
	output_json TEXT,
	error_text TEXT,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (node_id) REFERENCES node_registry(node_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS subagent_runs (
	run_id TEXT PRIMARY KEY,
	parent_session_key TEXT,
	parent_run_id TEXT,
	agent_id TEXT,
	child_session_key TEXT,
	child_session_id TEXT,
	task TEXT,
	label TEXT,
	model TEXT,
	thinking TEXT,
	caller_model TEXT,
	caller_thinking TEXT,
	cleanup_policy TEXT,
	run_timeout_seconds INTEGER,
	result_text TEXT,
	notes TEXT,
	runtime_ms INTEGER,
	usage_prompt_tokens INTEGER,
	usage_completion_tokens INTEGER,
	usage_total_tokens INTEGER,
	transcript_path TEXT,
	status TEXT,
	summary TEXT,
	error_text TEXT,
	announce_key TEXT,
	announce_attempts INTEGER,
	announce_sent_at TIMESTAMP,
	finished_at TIMESTAMP,
	archived_at TIMESTAMP,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
`
	if _, err := db.ExecContext(ctx, schema); err != nil {
		return err
	}
	if err := ensureSQLiteColumns(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, librarySchemaSQL); err != nil {
		return err
	}
	if err := createMemoryChunksFTSTable(ctx, db); err != nil {
		return err
	}
	return nil
}

func ensureSQLiteColumns(ctx context.Context, db *sql.DB) error {
	threadLastInteractivePresent := false
	providerCompatibilityPresent := false
	updates := []struct {
		table     string
		column    string
		statement string
	}{
		{
			table:     "sprites",
			column:    "source_type",
			statement: "ALTER TABLE sprites ADD COLUMN source_type TEXT NOT NULL DEFAULT ''",
		},
		{
			table:     "sprites",
			column:    "origin",
			statement: "ALTER TABLE sprites ADD COLUMN origin TEXT NOT NULL DEFAULT ''",
		},
		{
			table:     "gateway_sessions",
			column:    "context_summary",
			statement: "ALTER TABLE gateway_sessions ADD COLUMN context_summary TEXT",
		},
		{
			table:     "gateway_sessions",
			column:    "context_first_kept_message_id",
			statement: "ALTER TABLE gateway_sessions ADD COLUMN context_first_kept_message_id TEXT",
		},
		{
			table:     "gateway_sessions",
			column:    "context_strategy_version",
			statement: "ALTER TABLE gateway_sessions ADD COLUMN context_strategy_version INTEGER",
		},
		{
			table:     "gateway_sessions",
			column:    "context_compacted_at",
			statement: "ALTER TABLE gateway_sessions ADD COLUMN context_compacted_at TIMESTAMP",
		},
		{
			table:     "settings",
			column:    "color_scheme",
			statement: "ALTER TABLE settings ADD COLUMN color_scheme TEXT",
		},
		{
			table:     "settings",
			column:    "appearance_config_json",
			statement: "ALTER TABLE settings ADD COLUMN appearance_config_json TEXT",
		},
		{
			table:     "telemetry_state",
			column:    "distinct_days_used",
			statement: "ALTER TABLE telemetry_state ADD COLUMN distinct_days_used INTEGER NOT NULL DEFAULT 0",
		},
		{
			table:     "telemetry_state",
			column:    "distinct_days_used_last_month",
			statement: "ALTER TABLE telemetry_state ADD COLUMN distinct_days_used_last_month INTEGER NOT NULL DEFAULT 0",
		},
		{
			table:     "telemetry_state",
			column:    "completed_session_count",
			statement: "ALTER TABLE telemetry_state ADD COLUMN completed_session_count INTEGER NOT NULL DEFAULT 0",
		},
		{
			table:     "telemetry_state",
			column:    "total_session_seconds",
			statement: "ALTER TABLE telemetry_state ADD COLUMN total_session_seconds REAL NOT NULL DEFAULT 0",
		},
		{
			table:     "telemetry_state",
			column:    "previous_session_seconds",
			statement: "ALTER TABLE telemetry_state ADD COLUMN previous_session_seconds REAL",
		},
		{
			table:  "threads",
			column: "last_interactive_at",
			// SQLite rejects ALTER TABLE ... ADD COLUMN with non-constant defaults such as
			// CURRENT_TIMESTAMP, so legacy databases must add this column without a default
			// and then backfill from updated_at.
			statement: "ALTER TABLE threads ADD COLUMN last_interactive_at TIMESTAMP",
		},
		{
			table:     "thread_messages",
			column:    "kind",
			statement: "ALTER TABLE thread_messages ADD COLUMN kind TEXT NOT NULL DEFAULT 'chat'",
		},
		{
			table:     "providers",
			column:    "compatibility",
			statement: "ALTER TABLE providers ADD COLUMN compatibility TEXT NOT NULL DEFAULT ''",
		},
		{
			table:     "library_files",
			column:    "metadata_json",
			statement: "ALTER TABLE library_files ADD COLUMN metadata_json TEXT",
		},
		{
			table:     "library_files",
			column:    "display_name",
			statement: "ALTER TABLE library_files ADD COLUMN display_name TEXT",
		},
	}
	for _, item := range updates {
		hasTable, err := sqliteTableExists(ctx, db, item.table)
		if err != nil {
			return err
		}
		if !hasTable {
			continue
		}
		hasColumn, err := sqliteTableHasColumn(ctx, db, item.table, item.column)
		if err != nil {
			return err
		}
		if hasColumn {
			if item.table == "threads" && item.column == "last_interactive_at" {
				threadLastInteractivePresent = true
			}
			if item.table == "providers" && item.column == "compatibility" {
				providerCompatibilityPresent = true
			}
			continue
		}
		if _, err := db.ExecContext(ctx, item.statement); err != nil {
			return err
		}
		if item.table == "threads" && item.column == "last_interactive_at" {
			threadLastInteractivePresent = true
		}
		if item.table == "providers" && item.column == "compatibility" {
			providerCompatibilityPresent = true
		}
	}
	if threadLastInteractivePresent {
		if _, err := db.ExecContext(ctx, "UPDATE threads SET last_interactive_at = updated_at WHERE last_interactive_at IS NULL"); err != nil {
			return err
		}
	}
	if providerCompatibilityPresent {
		if _, err := db.ExecContext(ctx, `
UPDATE providers
SET compatibility = CASE
	WHEN id = 'deepseek' THEN 'deepseek'
	WHEN id = 'openrouter' THEN 'openrouter'
	WHEN id = 'google' THEN 'google'
	WHEN type = 'anthropic' THEN 'anthropic'
	ELSE 'openai'
END
WHERE TRIM(COALESCE(compatibility, '')) = ''
`); err != nil {
			return err
		}
	}
	if err := backfillTelemetrySessionDays(ctx, db); err != nil {
		return err
	}
	if err := backfillLibraryFileIdentity(ctx, db); err != nil {
		return err
	}
	return nil
}

func backfillTelemetrySessionDays(ctx context.Context, db *sql.DB) error {
	hasStateTable, err := sqliteTableExists(ctx, db, "telemetry_state")
	if err != nil || !hasStateTable {
		return err
	}
	hasDaysTable, err := sqliteTableExists(ctx, db, "telemetry_session_days")
	if err != nil || !hasDaysTable {
		return err
	}
	hasDistinctDaysUsed, err := sqliteTableHasColumn(ctx, db, "telemetry_state", "distinct_days_used")
	if err != nil || !hasDistinctDaysUsed {
		return err
	}
	hasDistinctDaysUsedLastMonth, err := sqliteTableHasColumn(ctx, db, "telemetry_state", "distinct_days_used_last_month")
	if err != nil || !hasDistinctDaysUsedLastMonth {
		return err
	}

	if _, err := db.ExecContext(ctx, `
INSERT OR IGNORE INTO telemetry_session_days(day, first_seen_at)
SELECT strftime('%Y-%m-%d', install_created_at), install_created_at
FROM telemetry_state
WHERE id = 1 AND launch_count > 0 AND TRIM(COALESCE(install_id, '')) <> ''
`); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, `
UPDATE telemetry_state
SET
	distinct_days_used = (SELECT COUNT(*) FROM telemetry_session_days),
	distinct_days_used_last_month = (
		SELECT COUNT(*)
		FROM telemetry_session_days
		WHERE day >= strftime('%Y-%m-%d', datetime('now', '-1 month'))
	)
WHERE id = 1
`); err != nil {
		return err
	}
	return nil
}

func backfillLibraryFileIdentity(ctx context.Context, db *sql.DB) error {
	hasTable, err := sqliteTableExists(ctx, db, "library_files")
	if err != nil || !hasTable {
		return err
	}
	hasDisplayName, err := sqliteTableHasColumn(ctx, db, "library_files", "display_name")
	if err != nil || !hasDisplayName {
		return err
	}

	rows, err := db.QueryContext(ctx, "SELECT id, name, display_name, storage_local_path FROM library_files")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id           string
			name         string
			displayName  sql.NullString
			storageLocal sql.NullString
		)
		if err := rows.Scan(&id, &name, &displayName, &storageLocal); err != nil {
			return err
		}

		nextDisplayName := strings.TrimSpace(displayName.String)
		if nextDisplayName == "" {
			nextDisplayName = strings.TrimSpace(name)
		}
		nextName := deriveLibraryStoredName(storageLocal.String, name)

		if nextName == strings.TrimSpace(name) && nextDisplayName == strings.TrimSpace(displayName.String) {
			continue
		}
		if _, err := db.ExecContext(ctx, "UPDATE library_files SET name = ?, display_name = ? WHERE id = ?", nextName, nextDisplayName, id); err != nil {
			return err
		}
	}
	return rows.Err()
}

const librarySchemaSQL = `
CREATE TABLE IF NOT EXISTS library_libraries (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  created_by_json TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS library_module_config (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  config_json TEXT NOT NULL,
  updated_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS library_files (
  id TEXT PRIMARY KEY,
  library_id TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('video','audio','subtitle','thumbnail','transcode')),
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
    (kind IN ('video','audio','thumbnail','transcode') AND storage_mode IN ('local_path','hybrid') AND COALESCE(storage_local_path,'') <> '') OR
    (kind = 'subtitle' AND storage_mode IN ('db_document','hybrid') AND COALESCE(storage_document_id,'') <> '')
  ),
  CHECK (
    (origin_kind = 'import' AND COALESCE(origin_import_path,'') <> '' AND origin_operation_id IS NULL) OR
    (origin_kind IN ('download','transcode') AND COALESCE(origin_operation_id,'') <> '' AND origin_import_path IS NULL)
  )
);

CREATE TABLE IF NOT EXISTS library_subtitle_documents (
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

CREATE TABLE IF NOT EXISTS dreamfm_local_tracks (
  file_id TEXT PRIMARY KEY,
  library_id TEXT NOT NULL,
  local_path TEXT NOT NULL,
  title TEXT NOT NULL,
  author TEXT,
  cover_local_path TEXT,
  format TEXT,
  audio_codec TEXT,
  duration_ms INTEGER,
  size_bytes INTEGER,
  mod_time_unix INTEGER NOT NULL DEFAULT 0,
  availability TEXT NOT NULL CHECK (availability IN ('available','missing')),
  last_checked_at TIMESTAMP NOT NULL,
  probe_error TEXT,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  FOREIGN KEY (file_id) REFERENCES library_files(id) ON DELETE CASCADE,
  FOREIGN KEY (library_id) REFERENCES library_libraries(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS dreamfm_local_tracks_available_idx
  ON dreamfm_local_tracks(updated_at DESC, title COLLATE NOCASE)
  WHERE availability = 'available';

CREATE INDEX IF NOT EXISTS dreamfm_local_tracks_library_idx
  ON dreamfm_local_tracks(library_id, updated_at DESC);

CREATE INDEX IF NOT EXISTS dreamfm_local_tracks_path_idx
  ON dreamfm_local_tracks(local_path);

CREATE TABLE IF NOT EXISTS library_operations (
  id TEXT PRIMARY KEY,
  library_id TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('download','transcode')),
  status TEXT NOT NULL CHECK (status IN ('queued','running','succeeded','failed','canceled')),
  display_name TEXT NOT NULL,

  correlation_json TEXT NOT NULL,
  input_json TEXT NOT NULL,
  output_json TEXT NOT NULL,
  meta_json TEXT,
  progress_json TEXT,

  source_domain TEXT,
  source_icon TEXT,
  file_count INTEGER NOT NULL DEFAULT 0,
  total_size_bytes INTEGER,
  duration_ms INTEGER,
  error_code TEXT,
  error_message TEXT,
  created_at TIMESTAMP NOT NULL,
  started_at TIMESTAMP,
  finished_at TIMESTAMP,

  FOREIGN KEY (library_id) REFERENCES library_libraries(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS library_operation_outputs (
  id TEXT PRIMARY KEY,
  operation_id TEXT NOT NULL,
  library_id TEXT NOT NULL,
  file_id TEXT NOT NULL,
  is_primary BOOLEAN NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL,
  FOREIGN KEY (operation_id) REFERENCES library_operations(id) ON DELETE CASCADE,
  FOREIGN KEY (library_id) REFERENCES library_libraries(id) ON DELETE CASCADE,
  FOREIGN KEY (file_id) REFERENCES library_files(id) ON DELETE CASCADE,
  UNIQUE (operation_id, file_id)
);

CREATE TABLE IF NOT EXISTS library_operation_chunks (
  id TEXT PRIMARY KEY,
  operation_id TEXT NOT NULL,
  library_id TEXT NOT NULL,
  chunk_index INTEGER NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('queued','running','succeeded','failed','canceled')),
  source_range TEXT,
  input_hash TEXT,
  request_hash TEXT,
  prompt_hash TEXT,
  response_hash TEXT,
  result_json TEXT,
  usage_json TEXT,
  retry_count INTEGER NOT NULL DEFAULT 0,
  error_message TEXT,
  started_at TIMESTAMP,
  finished_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  FOREIGN KEY (operation_id) REFERENCES library_operations(id) ON DELETE CASCADE,
  FOREIGN KEY (library_id) REFERENCES library_libraries(id) ON DELETE CASCADE,
  UNIQUE (operation_id, chunk_index)
);

CREATE TABLE IF NOT EXISTS library_history_records (
  id TEXT PRIMARY KEY,
  library_id TEXT NOT NULL,
  category TEXT NOT NULL CHECK (category IN ('operation','import')),
  action TEXT NOT NULL,
  display_name TEXT NOT NULL,
  status TEXT NOT NULL,

  source_kind TEXT NOT NULL,
  source_caller TEXT,
  source_run_id TEXT,
  source_actor TEXT,

  operation_id TEXT,
  import_batch_id TEXT,

  file_count INTEGER NOT NULL DEFAULT 0,
  total_size_bytes INTEGER,
  duration_ms INTEGER,

  import_path TEXT,
  keep_source_file BOOLEAN,
  error_code TEXT,
  error_message TEXT,

  occurred_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,

  FOREIGN KEY (library_id) REFERENCES library_libraries(id) ON DELETE CASCADE,
  FOREIGN KEY (operation_id) REFERENCES library_operations(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS library_history_files (
  id TEXT PRIMARY KEY,
  history_id TEXT NOT NULL,
  file_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  format TEXT,
  size_bytes INTEGER,
  deleted BOOLEAN NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL,
  FOREIGN KEY (history_id) REFERENCES library_history_records(id) ON DELETE CASCADE,
  FOREIGN KEY (file_id) REFERENCES library_files(id) ON DELETE CASCADE,
  UNIQUE (history_id, file_id)
);

CREATE TABLE IF NOT EXISTS library_workspace_states (
  id TEXT PRIMARY KEY,
  library_id TEXT NOT NULL,
  state_version INTEGER NOT NULL,
  state_json TEXT NOT NULL,
  operation_id TEXT,
  created_at TIMESTAMP NOT NULL,
  FOREIGN KEY (library_id) REFERENCES library_libraries(id) ON DELETE CASCADE,
  FOREIGN KEY (operation_id) REFERENCES library_operations(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS library_workspace_state_head (
  library_id TEXT PRIMARY KEY,
  latest_state_id TEXT NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  FOREIGN KEY (library_id) REFERENCES library_libraries(id) ON DELETE CASCADE,
  FOREIGN KEY (latest_state_id) REFERENCES library_workspace_states(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS library_file_events (
  id TEXT PRIMARY KEY,
  library_id TEXT NOT NULL,
  file_id TEXT NOT NULL,
  operation_id TEXT,
  event_type TEXT NOT NULL,
  detail_json TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL,
  FOREIGN KEY (library_id) REFERENCES library_libraries(id) ON DELETE CASCADE,
  FOREIGN KEY (file_id) REFERENCES library_files(id) ON DELETE CASCADE,
  FOREIGN KEY (operation_id) REFERENCES library_operations(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS library_files_library_created_idx ON library_files(library_id, created_at DESC);
CREATE INDEX IF NOT EXISTS library_operations_library_created_idx ON library_operations(library_id, created_at DESC);
CREATE INDEX IF NOT EXISTS library_operation_chunks_operation_idx ON library_operation_chunks(operation_id, chunk_index ASC);
CREATE INDEX IF NOT EXISTS library_history_library_occurred_idx ON library_history_records(library_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS library_events_library_created_idx ON library_file_events(library_id, created_at DESC);

CREATE TRIGGER IF NOT EXISTS trg_operation_output_same_library
BEFORE INSERT ON library_operation_outputs
FOR EACH ROW
BEGIN
  SELECT CASE
    WHEN NEW.library_id != (SELECT library_id FROM library_operations WHERE id = NEW.operation_id)
      THEN RAISE(ABORT, 'operation output library mismatch')
  END;

  SELECT CASE
    WHEN NEW.library_id != (SELECT library_id FROM library_files WHERE id = NEW.file_id)
      THEN RAISE(ABORT, 'file library mismatch')
  END;
END;
`
