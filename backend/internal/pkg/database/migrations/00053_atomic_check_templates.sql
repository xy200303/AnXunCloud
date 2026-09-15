-- 00053：检查项模板按甲方 9.14 实测定稿落地为原子模板（对齐官方月报明细表列）：
--   消火栓箱(精准6项) / 灭火器(精准5项) / 应急疏散类(粗放6项) / 报警喷淋类(粗放3项)，
-- 全部 photo_mode=group（整组 1 张拍照一次 AI 识别），逐项 photo_required=none。
-- 每租户一套（幂等）；旧合并模板「消火栓及灭火器检查」的点位关联重绑到 消火栓箱+灭火器 后停用。
-- 注意：Down 无法还原旧关联（重绑后旧关联行已删），仅删新模板并恢复旧模板启用状态。

-- +goose Up

-- 1. 四个原子模板（每租户一套；point_type 空=通用）
INSERT INTO public.check_template (id, tenant_id, name, point_type, photo_mode, sort, status, remark, created_at, updated_at)
SELECT gen_random_uuid(), t.id, x.name, '', 'group', x.sort, 'enabled', x.remark, now(), now()
FROM public.tenant t
JOIN (VALUES
    ('消火栓箱',   10, '官方月报4.2明细列：箱门/水带/枪头/接口/水压/周围（整组拍照一次识别）'),
    ('灭火器',     20, '官方月报4.1明细列：压力/瓶体/喷管/铅封/有效期（整组拍照一次识别）'),
    ('应急疏散类', 30, '官方月报4.3明细列：应急灯/疏散指示/安全出口/防火门/卷帘门/通道（粗放，每月抽查）'),
    ('报警喷淋类', 40, '手报/烟感/喷淋头（粗放，每月抽查）')
) AS x(name, sort, remark) ON true
WHERE NOT EXISTS (
      SELECT 1 FROM public.check_template c
      WHERE c.tenant_id = t.id AND c.name = x.name AND c.deleted_at IS NULL
  );

-- 2. 模板检查项（requirement 与 ai_hint 同文：判定标准即识别要点；judge_type 按语义归类）
INSERT INTO public.check_template_item (id, template_id, name, requirement, ai_hint, judge_type, required, photo_required, sort, created_at)
SELECT gen_random_uuid(), c.id, i.item_name, i.req, i.req, i.jt, true, 'none', i.sort, now()
FROM (VALUES
    -- 消火栓箱（官方月报 4.2 列）
    ('消火栓箱', 1, '箱门完好无损',   '箱门无破损、无变形、开闭正常，箱面无锈蚀', 'damage'),
    ('消火栓箱', 2, '水带齐全无破损', '消防水带在位、数量齐全，无破损、无霉变',   'presence'),
    ('消火栓箱', 3, '枪头齐全在位',   '水枪枪头在位、无缺失',                     'presence'),
    ('消火栓箱', 4, '接口完好',       '水带接口与栓口接口无锈蚀、卡扣完好、无渗漏', 'damage'),
    ('消火栓箱', 5, '水压正常',       '栓口水压正常，无异常泄压（结合压力表/试水判断）', 'state'),
    ('消火栓箱', 6, '周围无遮挡',     '消火栓箱前无杂物堆放、无遮挡，取用通道畅通', 'passage'),
    -- 灭火器（官方月报 4.1 列）
    ('灭火器',   1, '压力正常',       '压力表指针在绿色区域',                     'indicator'),
    ('灭火器',   2, '瓶体完好',       '瓶体无破损、无明显锈蚀、无变形',           'damage'),
    ('灭火器',   3, '喷管完好',       '喷管无龟裂、无老化、无堵塞，喷嘴完好',     'damage'),
    ('灭火器',   4, '铅封完好',       '铅封/保险销在位完好、未被拆除',            'presence'),
    ('灭火器',   5, '在有效期内',     '生产日期/维修（换粉）日期标签清晰，未超出有效期', 'label'),
    -- 应急疏散类（官方月报 4.3 列，粗放）
    ('应急疏散类', 1, '应急照明灯完好', '灯具外观完好、通电正常、断电可点亮',      'general'),
    ('应急疏散类', 2, '疏散指示灯完好', '指示灯常亮、方向指示正确、无破损',        'general'),
    ('应急疏散类', 3, '安全出口标示清晰', '安全出口标识清晰、无遮挡',              'general'),
    ('应急疏散类', 4, '防火门开闭正常', '防火门开闭正常、闭门器有效、无损坏',      'state'),
    ('应急疏散类', 5, '防火卷帘门正常', '防火卷帘门外观完好、导轨无阻碍',          'state'),
    ('应急疏散类', 6, '消防通道畅通',   '通道无杂物堆放、畅通无阻',                'passage'),
    -- 报警喷淋类（粗放，每月抽查）
    ('报警喷淋类', 1, '手动报警按钮完好', '按钮外观完好、面板无破损、标识清晰',    'damage'),
    ('报警喷淋类', 2, '烟感探测器无异常', '烟感外观完好、无遮挡、无脱落',          'general'),
    ('报警喷淋类', 3, '喷淋头完好无遮挡', '喷淋头无破损、无涂覆、周围无遮挡',      'damage')
) AS i(tpl_name, sort, item_name, req, jt)
JOIN public.check_template c ON c.name = i.tpl_name AND c.deleted_at IS NULL
WHERE NOT EXISTS (
    SELECT 1 FROM public.check_template_item ci
    WHERE ci.template_id = c.id AND ci.name = i.item_name
);

-- 3. 旧合并模板「消火栓及灭火器检查」的点位关联 → 重绑 消火栓箱 + 灭火器
INSERT INTO public.point_template (id, point_id, template_id, sort, created_at)
SELECT gen_random_uuid(), pt.point_id, c.id, 10, now()
FROM public.point_template pt
JOIN public.check_template old ON old.id = pt.template_id AND old.name = '消火栓及灭火器检查' AND old.deleted_at IS NULL
JOIN public.check_template c ON c.tenant_id IS NOT DISTINCT FROM old.tenant_id AND c.name = '消火栓箱' AND c.deleted_at IS NULL
ON CONFLICT DO NOTHING;

INSERT INTO public.point_template (id, point_id, template_id, sort, created_at)
SELECT gen_random_uuid(), pt.point_id, c.id, 20, now()
FROM public.point_template pt
JOIN public.check_template old ON old.id = pt.template_id AND old.name = '消火栓及灭火器检查' AND old.deleted_at IS NULL
JOIN public.check_template c ON c.tenant_id IS NOT DISTINCT FROM old.tenant_id AND c.name = '灭火器' AND c.deleted_at IS NULL
ON CONFLICT DO NOTHING;

-- 4. 清掉旧关联并停用旧合并模板（历史打卡记录为快照，不受影响）
DELETE FROM public.point_template pt
USING public.check_template old
WHERE pt.template_id = old.id AND old.name = '消火栓及灭火器检查';

UPDATE public.check_template SET status = 'disabled', updated_at = now()
WHERE name = '消火栓及灭火器检查' AND deleted_at IS NULL;

-- +goose Down
DELETE FROM public.point_template pt
USING public.check_template c
WHERE pt.template_id = c.id AND c.name IN ('消火栓箱', '灭火器', '应急疏散类', '报警喷淋类');
DELETE FROM public.check_template_item ci
USING public.check_template c
WHERE ci.template_id = c.id AND c.name IN ('消火栓箱', '灭火器', '应急疏散类', '报警喷淋类');
DELETE FROM public.check_template WHERE name IN ('消火栓箱', '灭火器', '应急疏散类', '报警喷淋类');
UPDATE public.check_template SET status = 'enabled', updated_at = now()
WHERE name = '消火栓及灭火器检查' AND deleted_at IS NULL;
