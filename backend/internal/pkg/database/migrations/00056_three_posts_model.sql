-- 00056：三岗位极简模型——拆除巡查汇报线维度槽位机制，清理存量数据。
-- 背景：维度槽位（patrol_report_line.<patrol_type>）的平台默认绑定（fire/equipment/environment/building
-- → 工程/环境/客服主管）是硬编码的"别人家物业"组织假设，会在项目内无人挂该岗位时静默截胡
-- 通用汇报线配置，导致审核链名单为空、谁都审不了。维度槽位路由代码已整体删除，
-- 本迁移清理存量：所有维度槽位绑定（各作用域；无人显式配置过，均系种子复制）+
-- 未被引用的非三岗位（project_manager/safety_supervisor/inspector）岗位模板行。

-- +goose Up

-- 1. 维度槽位绑定全量清除（平台级/租户级/项目级；机制已删除，这些行不再被读取）
DELETE FROM public.duty_binding WHERE slot LIKE 'patrol_report_line.%';

-- 2. 删除未被编制/绑定引用的非三岗位岗位行（平台模板与租户复制行同规则；
--    jsonb 数组成员判定用 ? 运算符；被 project_staff.posts 或 duty_binding.post_codes 引用的保留）
DELETE FROM public.post_dict p
WHERE p.code NOT IN ('project_manager', 'safety_supervisor', 'inspector')
  AND NOT EXISTS (SELECT 1 FROM public.project_staff ps WHERE ps.posts ? p.code)
  AND NOT EXISTS (SELECT 1 FROM public.duty_binding b WHERE b.post_codes ? p.code);

-- +goose Down

-- 数据删除类迁移，不可还原（维度槽位机制已随代码删除，无还原意义）。
