-- 00036：平台管理新增「应用发布」菜单（/platform/releases），发布物管理从「品牌官网」拆出。
-- 存量库迁移：删旧按钮 → 插新菜单+按钮 → 超管授权。全部幂等，可重复执行。
-- 全新库由 seed.go 菜单树直接生成（本迁移 NOT EXISTS 守卫自动跳过）。

-- +goose Up

-- 1) 删除「品牌官网」下的上传/删除发布物按钮（含角色授权关联）
DELETE FROM sys_role_menu WHERE menu_id IN (
  SELECT id FROM sys_menu WHERE perms IN ('system:site:upload', 'system:site:delete')
    AND parent_id = (SELECT id FROM sys_menu WHERE path = '/platform/site' AND deleted_at IS NULL)
);
DELETE FROM sys_menu WHERE perms IN ('system:site:upload', 'system:site:delete')
  AND parent_id = (SELECT id FROM sys_menu WHERE path = '/platform/site' AND deleted_at IS NULL);

-- 2) 新增「应用发布」菜单（平台级，仅超管可见）
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform, created_at, updated_at)
SELECT gen_random_uuid(), p.id, '应用发布', '/platform/releases', 'Iphone', 'menu', 'system:site:list', 6, true, 'enabled', true, true, now(), now()
FROM sys_menu p
WHERE p.path = '/platform' AND p.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM sys_menu m WHERE m.path = '/platform/releases' AND m.deleted_at IS NULL);

-- 3) 新菜单下的两个按钮
INSERT INTO sys_menu (id, parent_id, title, path, icon, type, perms, sort, visible, status, is_builtin, is_platform, created_at, updated_at)
SELECT gen_random_uuid(), m.id, t.title, '', '', 'button', t.perms, t.sort, false, 'enabled', true, true, now(), now()
FROM (SELECT id FROM sys_menu WHERE path = '/platform/releases' AND deleted_at IS NULL) m
CROSS JOIN (VALUES
  ('上传发布物', 'system:site:upload', 1),
  ('删除发布物', 'system:site:delete', 2)
) AS t(title, perms, sort)
WHERE NOT EXISTS (
  SELECT 1 FROM sys_menu b WHERE b.parent_id = m.id AND b.perms = t.perms AND b.deleted_at IS NULL
);

-- 4) 超管授权新菜单与按钮
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT r.id, m.id
FROM sys_role r, sys_menu m
WHERE r.code = 'super_admin'
  AND (m.path = '/platform/releases' OR m.parent_id = (SELECT id FROM sys_menu WHERE path = '/platform/releases' AND deleted_at IS NULL))
  AND m.deleted_at IS NULL
ON CONFLICT (role_id, menu_id) DO NOTHING;

-- +goose Down
-- 还原：新菜单连按钮删除（role_menu 由外键级联清理），旧按钮结构不回补（按钮权限点不变，影响为零）。
DELETE FROM sys_menu WHERE path = '/platform/releases'
   OR parent_id = (SELECT id FROM sys_menu WHERE path = '/platform/releases');
