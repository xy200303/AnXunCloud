package database

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// migration 单条迁移：版本号 + SQL 脚本。
type migration struct {
	Version int
	Name    string
	SQL     string
}

// migrations 全量迁移脚本。v1 与《数据库设计文档》第三章 DDL 一一对应。
var migrations = []migration{
	{Version: 1, Name: "init_schema", SQL: initSchemaSQL},
	{Version: 2, Name: "sys_user_must_change_password", SQL: `
ALTER TABLE sys_user ADD COLUMN IF NOT EXISTS must_change_password boolean NOT NULL DEFAULT false;
COMMENT ON COLUMN sys_user.must_change_password IS '首次登录强制修改密码标记（批量导入用户置 true）';
`},
	{Version: 3, Name: "builtin_flags", SQL: `
ALTER TABLE sys_role ADD COLUMN IF NOT EXISTS is_builtin boolean NOT NULL DEFAULT false;
ALTER TABLE sys_menu ADD COLUMN IF NOT EXISTS is_builtin boolean NOT NULL DEFAULT false;
COMMENT ON COLUMN sys_role.is_builtin IS '内置角色不可删除（错误码 41007）';
COMMENT ON COLUMN sys_menu.is_builtin IS '内置菜单不可删除（错误码 41007）';
`},
	{Version: 4, Name: "phase2_tables", SQL: phase2SQL},
	{Version: 5, Name: "casbin_rule", SQL: `
-- Casbin 策略表（RBAC with domains，见 internal/pkg/authz；gorm-adapter 也会自动建表，此处显式迁移）
CREATE TABLE IF NOT EXISTS casbin_rule (
    id    bigserial PRIMARY KEY,
    ptype varchar(100) NULL,
    v0    varchar(100) NULL,
    v1    varchar(100) NULL,
    v2    varchar(100) NULL,
    v3    varchar(100) NULL,
    v4    varchar(100) NULL,
    v5    varchar(100) NULL
);
COMMENT ON TABLE casbin_rule IS 'Casbin 策略表（p：角色→权限点；g：用户→角色，域 default）';
`},
	{Version: 6, Name: "casbin_rule_rebuild", SQL: `
-- 策略模型调整为三元组（sub, dom, obj=完整权限点字符串）：casbin_rule 为派生数据，清空后由启动 SyncAll 重建
DELETE FROM casbin_rule;
`},
	{Version: 7, Name: "config_register_enabled", SQL: `
-- 开放注册开关（存量库幂等补充；id 内联生成，见 v4 说明）
INSERT INTO sys_config (id, key, name, value, remark)
SELECT gen_random_uuid(), 'auth.register_enabled', '是否开放注册', 'false', '开启后允许通过 /api/admin/auth/register 自助注册后台账号'
WHERE NOT EXISTS (SELECT 1 FROM sys_config WHERE key = 'auth.register_enabled');
`},
	{Version: 8, Name: "sys_user_is_builtin", SQL: `
-- 内置账号标记：唯一超级管理员 admin 不可删除/停用/移除超管角色
ALTER TABLE sys_user ADD COLUMN IF NOT EXISTS is_builtin boolean NOT NULL DEFAULT false;
COMMENT ON COLUMN sys_user.is_builtin IS '内置账号（admin）：禁止删除/停用/移除 super_admin 角色（错误码 41014）';
-- 存量库：admin 账号或持有 super_admin 角色的账号标记为内置
UPDATE sys_user SET is_builtin = true
WHERE username = 'admin'
   OR role_ids @> ('["' || (SELECT id FROM sys_role WHERE code = 'super_admin' AND deleted_at IS NULL LIMIT 1) || '"]')::jsonb;
`},
}

// Migrate 执行未应用过的迁移，并确保当月与下月分区存在。
func Migrate(db *gorm.DB) error {
	// 迁移记录表
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    int PRIMARY KEY,
		name       varchar(128) NOT NULL,
		applied_at timestamptz  NOT NULL DEFAULT now()
	)`).Error; err != nil {
		return err
	}

	for _, m := range migrations {
		var count int64
		if err := db.Raw(`SELECT count(*) FROM schema_migrations WHERE version = ?`, m.Version).Scan(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(m.SQL).Error; err != nil {
				return fmt.Errorf("迁移 v%d(%s) 执行失败: %w", m.Version, m.Name, err)
			}
			return tx.Exec(`INSERT INTO schema_migrations (version, name) VALUES (?, ?)`, m.Version, m.Name).Error
		}); err != nil {
			return err
		}
	}

	// 按月分区表：确保当月与下月分区已创建（月度滚动由运维脚本兜底，这里保证启动即可写）
	if err := EnsurePartitions(db, "checkin_record"); err != nil {
		return err
	}
	return EnsurePartitions(db, "sys_operation_log")
}

// EnsurePartitions 为分区表创建当月与下月分区（不存在才建）。
func EnsurePartitions(db *gorm.DB, parent string) error {
	now := time.Now()
	for i := 0; i < 2; i++ {
		first := time.Date(now.Year(), now.Month()+time.Month(i), 1, 0, 0, 0, 0, time.Local)
		next := first.AddDate(0, 1, 0)
		partName := fmt.Sprintf("%s_%04d_%02d", parent, first.Year(), int(first.Month()))
		sql := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES FROM (%s) TO (%s)`,
			partName, parent,
			quoteLiteral(first.Format("2006-01-02")), quoteLiteral(next.Format("2006-01-02")))
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("创建分区 %s 失败: %w", partName, err)
		}
	}
	return nil
}

func quoteLiteral(s string) string { return "'" + s + "'" }
