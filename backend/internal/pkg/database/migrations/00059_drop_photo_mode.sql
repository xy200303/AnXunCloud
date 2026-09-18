-- 00059：删除模板拍照模式（photo_mode）——统一「一项一图」。
-- 整组模式（1 张整组照 AI 判多项）已由「单项 + 观察点 tags」取代（00057/00058）：
-- 一张照多点核对走 tags，group/per_item 不再有意义；App 端整组识别链路同步移除。幂等可重复执行。

-- +goose Up

ALTER TABLE check_template DROP COLUMN IF EXISTS photo_mode;

-- 整体项要求文案去掉「整组」措辞（00058 写入的文本）
UPDATE public.check_template_item SET requirement = '拍 1 张现场照片，逐项核对观察点；异常观察点在 App 上勾选上报'
WHERE requirement = '整组拍 1 张照片，逐项核对观察点；异常观察点在 App 上勾选上报';

-- +goose Down

ALTER TABLE check_template ADD COLUMN IF NOT EXISTS photo_mode varchar(16) NOT NULL DEFAULT 'group';
