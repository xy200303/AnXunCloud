-- 00051：点位多模板组合——inspection_point.template_id(单值) → point_template 关联表(多对多)，
-- 打卡/任务/AI/报告均按"点位全部模板的检查项并集"组装；存量数据一次性搬入后删除旧列（不做双写兼容）。
-- 同时 check_template 加 photo_mode（group=整组 1 张拍照一次 AI 识别多项 / per_item=逐项拍照）。

-- +goose Up
CREATE TABLE IF NOT EXISTS public.point_template (
    id          uuid PRIMARY KEY,
    point_id    uuid NOT NULL REFERENCES public.inspection_point(id) ON DELETE CASCADE,
    template_id uuid NOT NULL REFERENCES public.check_template(id) ON DELETE CASCADE,
    sort        int  NOT NULL DEFAULT 0,
    created_at  timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE public.point_template IS '点位-检查项模板关联（多对多；点位检查项 = 其全部模板检查项的并集）';
CREATE UNIQUE INDEX IF NOT EXISTS uq_point_template ON public.point_template (point_id, template_id);
CREATE INDEX IF NOT EXISTS idx_point_template_tpl ON public.point_template (template_id);

INSERT INTO public.point_template (id, point_id, template_id, sort, created_at)
SELECT gen_random_uuid(), id, template_id, 0, now() FROM public.inspection_point WHERE template_id IS NOT NULL;

ALTER TABLE public.inspection_point DROP COLUMN IF EXISTS template_id;

ALTER TABLE public.check_template ADD COLUMN IF NOT EXISTS photo_mode varchar(16) NOT NULL DEFAULT 'group';
COMMENT ON COLUMN public.check_template.photo_mode IS '拍照模式：group=整组 1 张照片一次 AI 识别多项（默认）；per_item=逐项拍照';

-- +goose Down
ALTER TABLE public.inspection_point ADD COLUMN IF NOT EXISTS template_id uuid;
UPDATE public.inspection_point p SET template_id = t.template_id
FROM (SELECT DISTINCT ON (point_id) point_id, template_id FROM public.point_template ORDER BY point_id, sort ASC) t
WHERE p.id = t.point_id;
ALTER TABLE public.check_template DROP COLUMN IF EXISTS photo_mode;
DROP TABLE IF EXISTS public.point_template;
