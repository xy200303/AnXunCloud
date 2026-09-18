-- 00058：检查项粒度收敛（甲方口径：一项一张照片，细分观察点收敛为项内 tag 数组）。
-- 00053 的四个原子模板（消火栓箱 6 项/灭火器 5 项/应急疏散类 6 项/报警喷淋类 3 项）各收敛为
-- 1 个整体检查项 + tags；点位多模板组合后一个点位 1~3 个检查项。
-- 历史打卡为快照不受影响；月报明细列由 tags 展开（report_pdf_data 同步支持）。幂等可重复执行。

-- +goose Up

-- ========== 消火栓箱 ==========
DELETE FROM public.check_template_item ci
USING public.check_template c
WHERE ci.template_id = c.id AND c.name = '消火栓箱' AND c.deleted_at IS NULL
  AND ci.name <> '消火栓箱整体检查';

INSERT INTO public.check_template_item (id, template_id, name, requirement, ai_hint, judge_type, required, photo_required, sort, tags, created_at)
SELECT gen_random_uuid(), c.id, '消火栓箱整体检查',
       '整组拍 1 张照片，逐项核对观察点；异常观察点在 App 上勾选上报',
       '逐观察点核对，异常观察点原名填入 abnormal_tags',
       'general', true, 'none', 1,
       '["箱门完好无损","水带齐全无破损","枪头齐全在位","接口完好","水压正常","周围无遮挡"]'::jsonb, now()
FROM public.check_template c
WHERE c.name = '消火栓箱' AND c.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM public.check_template_item ci WHERE ci.template_id = c.id AND ci.name = '消火栓箱整体检查');

UPDATE public.check_template_item ci SET
  requirement = '整组拍 1 张照片，逐项核对观察点；异常观察点在 App 上勾选上报',
  ai_hint = '逐观察点核对，异常观察点原名填入 abnormal_tags',
  judge_type = 'general', photo_required = 'none', sort = 1,
  tags = '["箱门完好无损","水带齐全无破损","枪头齐全在位","接口完好","水压正常","周围无遮挡"]'::jsonb
FROM public.check_template c
WHERE ci.template_id = c.id AND c.name = '消火栓箱' AND c.deleted_at IS NULL AND ci.name = '消火栓箱整体检查';

-- ========== 灭火器 ==========
DELETE FROM public.check_template_item ci
USING public.check_template c
WHERE ci.template_id = c.id AND c.name = '灭火器' AND c.deleted_at IS NULL
  AND ci.name <> '灭火器整体检查';

INSERT INTO public.check_template_item (id, template_id, name, requirement, ai_hint, judge_type, required, photo_required, sort, tags, created_at)
SELECT gen_random_uuid(), c.id, '灭火器整体检查',
       '整组拍 1 张照片，逐项核对观察点；异常观察点在 App 上勾选上报',
       '逐观察点核对，异常观察点原名填入 abnormal_tags',
       'general', true, 'none', 1,
       '["压力正常","瓶体完好","喷管完好","铅封完好","在有效期内"]'::jsonb, now()
FROM public.check_template c
WHERE c.name = '灭火器' AND c.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM public.check_template_item ci WHERE ci.template_id = c.id AND ci.name = '灭火器整体检查');

UPDATE public.check_template_item ci SET
  requirement = '整组拍 1 张照片，逐项核对观察点；异常观察点在 App 上勾选上报',
  ai_hint = '逐观察点核对，异常观察点原名填入 abnormal_tags',
  judge_type = 'general', photo_required = 'none', sort = 1,
  tags = '["压力正常","瓶体完好","喷管完好","铅封完好","在有效期内"]'::jsonb
FROM public.check_template c
WHERE ci.template_id = c.id AND c.name = '灭火器' AND c.deleted_at IS NULL AND ci.name = '灭火器整体检查';

-- ========== 应急疏散类 ==========
DELETE FROM public.check_template_item ci
USING public.check_template c
WHERE ci.template_id = c.id AND c.name = '应急疏散类' AND c.deleted_at IS NULL
  AND ci.name <> '疏散设施整体检查';

INSERT INTO public.check_template_item (id, template_id, name, requirement, ai_hint, judge_type, required, photo_required, sort, tags, created_at)
SELECT gen_random_uuid(), c.id, '疏散设施整体检查',
       '整组拍 1 张照片，逐项核对观察点；异常观察点在 App 上勾选上报',
       '逐观察点核对，异常观察点原名填入 abnormal_tags',
       'general', true, 'none', 1,
       '["应急照明灯完好","疏散指示灯完好","安全出口标示清晰","防火门开闭正常","防火卷帘门正常","消防通道畅通"]'::jsonb, now()
FROM public.check_template c
WHERE c.name = '应急疏散类' AND c.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM public.check_template_item ci WHERE ci.template_id = c.id AND ci.name = '疏散设施整体检查');

UPDATE public.check_template_item ci SET
  requirement = '整组拍 1 张照片，逐项核对观察点；异常观察点在 App 上勾选上报',
  ai_hint = '逐观察点核对，异常观察点原名填入 abnormal_tags',
  judge_type = 'general', photo_required = 'none', sort = 1,
  tags = '["应急照明灯完好","疏散指示灯完好","安全出口标示清晰","防火门开闭正常","防火卷帘门正常","消防通道畅通"]'::jsonb
FROM public.check_template c
WHERE ci.template_id = c.id AND c.name = '应急疏散类' AND c.deleted_at IS NULL AND ci.name = '疏散设施整体检查';

-- ========== 报警喷淋类 ==========
DELETE FROM public.check_template_item ci
USING public.check_template c
WHERE ci.template_id = c.id AND c.name = '报警喷淋类' AND c.deleted_at IS NULL
  AND ci.name <> '报警喷淋设施整体检查';

INSERT INTO public.check_template_item (id, template_id, name, requirement, ai_hint, judge_type, required, photo_required, sort, tags, created_at)
SELECT gen_random_uuid(), c.id, '报警喷淋设施整体检查',
       '整组拍 1 张照片，逐项核对观察点；异常观察点在 App 上勾选上报',
       '逐观察点核对，异常观察点原名填入 abnormal_tags',
       'general', true, 'none', 1,
       '["手动报警按钮完好","烟感探测器无异常","喷淋头完好无遮挡"]'::jsonb, now()
FROM public.check_template c
WHERE c.name = '报警喷淋类' AND c.deleted_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM public.check_template_item ci WHERE ci.template_id = c.id AND ci.name = '报警喷淋设施整体检查');

UPDATE public.check_template_item ci SET
  requirement = '整组拍 1 张照片，逐项核对观察点；异常观察点在 App 上勾选上报',
  ai_hint = '逐观察点核对，异常观察点原名填入 abnormal_tags',
  judge_type = 'general', photo_required = 'none', sort = 1,
  tags = '["手动报警按钮完好","烟感探测器无异常","喷淋头完好无遮挡"]'::jsonb
FROM public.check_template c
WHERE ci.template_id = c.id AND c.name = '报警喷淋类' AND c.deleted_at IS NULL AND ci.name = '报警喷淋设施整体检查';

-- +goose Down
-- 还原为 00053 的细分项（删除整体项，按原 VALUES 重建；point_template 关联不动）。
DELETE FROM public.check_template_item ci
USING public.check_template c
WHERE ci.template_id = c.id AND c.deleted_at IS NULL
  AND ci.name IN ('消火栓箱整体检查', '灭火器整体检查', '疏散设施整体检查', '报警喷淋设施整体检查');

INSERT INTO public.check_template_item (id, template_id, name, requirement, ai_hint, judge_type, required, photo_required, sort, created_at)
SELECT gen_random_uuid(), c.id, i.item_name, i.req, i.req, i.jt, true, 'none', i.sort, now()
FROM (VALUES
    ('消火栓箱', 1, '箱门完好无损',   '箱门无破损、无变形、开闭正常，箱面无锈蚀', 'damage'),
    ('消火栓箱', 2, '水带齐全无破损', '消防水带在位、数量齐全，无破损、无霉变',   'presence'),
    ('消火栓箱', 3, '枪头齐全在位',   '水枪枪头在位、无缺失',                     'presence'),
    ('消火栓箱', 4, '接口完好',       '水带接口与栓口接口无锈蚀、卡扣完好、无渗漏', 'damage'),
    ('消火栓箱', 5, '水压正常',       '栓口水压正常，无异常泄压（结合压力表/试水判断）', 'state'),
    ('消火栓箱', 6, '周围无遮挡',     '消火栓箱前无杂物堆放、无遮挡，取用通道畅通', 'passage'),
    ('灭火器',   1, '压力正常',       '压力表指针在绿色区域',                     'indicator'),
    ('灭火器',   2, '瓶体完好',       '瓶体无破损、无明显锈蚀、无变形',           'damage'),
    ('灭火器',   3, '喷管完好',       '喷管无龟裂、无老化、无堵塞，喷嘴完好',     'damage'),
    ('灭火器',   4, '铅封完好',       '铅封/保险销在位完好、未被拆除',            'presence'),
    ('灭火器',   5, '在有效期内',     '生产日期/维修（换粉）日期标签清晰，未超出有效期', 'label'),
    ('应急疏散类', 1, '应急照明灯完好', '灯具外观完好、通电正常、断电可点亮',      'general'),
    ('应急疏散类', 2, '疏散指示灯完好', '指示灯常亮、方向指示正确、无破损',        'general'),
    ('应急疏散类', 3, '安全出口标示清晰', '安全出口标识清晰、无遮挡',              'general'),
    ('应急疏散类', 4, '防火门开闭正常', '防火门开闭正常、闭门器有效、无损坏',      'state'),
    ('应急疏散类', 5, '防火卷帘门正常', '防火卷帘门外观完好、导轨无阻碍',          'state'),
    ('应急疏散类', 6, '消防通道畅通',   '通道无杂物堆放、畅通无阻',                'passage'),
    ('报警喷淋类', 1, '手动报警按钮完好', '按钮外观完好、面板无破损、标识清晰',    'damage'),
    ('报警喷淋类', 2, '烟感探测器无异常', '烟感外观完好、无遮挡、无脱落',          'general'),
    ('报警喷淋类', 3, '喷淋头完好无遮挡', '喷淋头无破损、无涂覆、周围无遮挡',      'damage')
) AS i(tpl_name, sort, item_name, req, jt)
JOIN public.check_template c ON c.name = i.tpl_name AND c.deleted_at IS NULL
WHERE NOT EXISTS (
    SELECT 1 FROM public.check_template_item ci
    WHERE ci.template_id = c.id AND ci.name = i.item_name
);
