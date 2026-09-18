-- 00060：检查项「拍照引导语」（guide）——向导显示给巡检员的拍摄动作指引（如「拍消火栓箱整体照片，
-- 打开箱门让水带枪头都在画面里」），替代标准检查术语直出；空=App 兜底显示「拍「项名」照片」。
-- 只引导拍摄，不进打卡快照（记录/报告/审核仍显示标准项名术语）。幂等可重复执行。

-- +goose Up

ALTER TABLE check_template_item ADD COLUMN IF NOT EXISTS guide text;
COMMENT ON COLUMN public.check_template_item.guide IS '拍照引导语（巡检员视角的拍摄动作指引；空=App 兜底「拍「项名」照片」）';

-- 四个官方整体项的引导语（按名称定位，不依赖 id）
UPDATE public.check_template_item SET guide = '拍消火栓箱整体照片，打开箱门让水带、枪头都在画面里' WHERE name = '消火栓箱整体检查' AND (guide IS NULL OR guide = '');
UPDATE public.check_template_item SET guide = '拍灭火器整体照片，压力表盘正对镜头' WHERE name = '灭火器整体检查' AND (guide IS NULL OR guide = '');
UPDATE public.check_template_item SET guide = '拍现场整体照片，把应急灯、疏散指示牌都拍进画面' WHERE name = '疏散设施整体检查' AND (guide IS NULL OR guide = '');
UPDATE public.check_template_item SET guide = '拍现场整体照片，对准报警按钮、烟感或喷淋头' WHERE name = '报警喷淋设施整体检查' AND (guide IS NULL OR guide = '');

-- +goose Down

ALTER TABLE check_template_item DROP COLUMN IF EXISTS guide;
