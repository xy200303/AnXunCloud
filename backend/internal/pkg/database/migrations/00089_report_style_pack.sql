-- +goose Up
-- owner: report
CREATE TABLE report_style_pack (
    id uuid PRIMARY KEY,
    code varchar(64) NOT NULL UNIQUE,
    name varchar(128) NOT NULL,
    config jsonb NOT NULL,
    status varchar(16) NOT NULL DEFAULT 'enabled',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO report_style_pack (id, code, name, config, status) VALUES
('89000000-0000-0000-0000-000000000001', 'client_baseline', '甲方基础样式', '{"font_family":"Noto Sans SC","title_size":36,"heading_size":24,"body_size":20,"accent_color":"1F4E79"}', 'enabled');

-- +goose Down
SELECT 1;
