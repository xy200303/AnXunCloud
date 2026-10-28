-- +goose Up
-- owner: stats
CREATE TABLE stats_task (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    plan_id uuid NOT NULL,
    plan_name varchar(128) NOT NULL DEFAULT '',
    community_id uuid NOT NULL,
    community_name varchar(128) NOT NULL DEFAULT '',
    inspector_id uuid NOT NULL,
    inspector_name varchar(64) NOT NULL DEFAULT '',
    patrol_type varchar(16) NOT NULL,
    task_date date NOT NULL,
    round_name varchar(32) NOT NULL DEFAULT '',
    time_window varchar(32) NOT NULL DEFAULT '',
    due_date date,
    status varchar(16) NOT NULL,
    point_ids jsonb NOT NULL DEFAULT '[]',
    total_points integer NOT NULL,
    done_points integer NOT NULL,
    blocked_points integer NOT NULL DEFAULT 0,
    started_at timestamptz,
    finished_at timestamptz,
    daily_min_rounds integer,
    source_updated_at timestamptz NOT NULL,
    projected_at timestamptz NOT NULL
);
CREATE INDEX idx_stats_task_date_scope ON stats_task(tenant_id, community_id, task_date);
CREATE INDEX idx_stats_task_inspector ON stats_task(inspector_id, task_date);

CREATE TABLE stats_checkin (
    id uuid PRIMARY KEY,
    task_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    community_id uuid NOT NULL,
    community_name varchar(128) NOT NULL DEFAULT '',
    point_id uuid NOT NULL,
    point_name varchar(128) NOT NULL DEFAULT '',
    inspector_id uuid NOT NULL,
    inspector_name varchar(64) NOT NULL DEFAULT '',
    checkin_time timestamptz NOT NULL,
    result varchar(16) NOT NULL,
    is_suspect boolean NOT NULL DEFAULT false,
    remark varchar(512) NOT NULL DEFAULT '',
    superseded_by uuid,
    source_updated_at timestamptz NOT NULL,
    projected_at timestamptz NOT NULL
);
CREATE INDEX idx_stats_checkin_time_scope ON stats_checkin(tenant_id, community_id, checkin_time);
CREATE INDEX idx_stats_checkin_inspector ON stats_checkin(inspector_id, checkin_time);

CREATE TABLE stats_config_alert (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    community_id uuid NOT NULL,
    community_name varchar(128) NOT NULL DEFAULT '',
    point_id uuid NOT NULL,
    point_name varchar(128) NOT NULL DEFAULT '',
    reason varchar(255) NOT NULL,
    started_at timestamptz NOT NULL,
    ended_at timestamptz,
    source_updated_at timestamptz NOT NULL,
    projected_at timestamptz NOT NULL
);
CREATE INDEX idx_stats_config_alert_scope ON stats_config_alert(tenant_id, community_id, ended_at);

CREATE TABLE stats_projection_meta (
    source varchar(32) PRIMARY KEY,
    last_rebuild_at timestamptz NOT NULL,
    source_updated_at timestamptz NOT NULL
);

-- +goose Down
SELECT 1;
