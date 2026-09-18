-- 00064：四个官方整体检查项 photo_required 'none'→'required'。
-- 00058 收敛模板时误标免拍：引导语/要求都是"拍 1 张现场照片"，required=true 且服务端提交校验要照片，
-- 导致手动档不渲染照片槽（按 photo_required=none 隐藏）、提交才被服务端拒。顺手把 requirement 里
-- 的"整组"（已删除的整组模式遗留措辞）改掉。幂等可重复执行。

-- +goose Up

UPDATE public.check_template_item SET
  photo_required = 'required',
  requirement = '拍 1 张现场照片，逐项核对观察点；异常观察点在 App 上勾选上报'
WHERE name IN ('消火栓箱整体检查', '灭火器整体检查', '疏散设施整体检查', '报警喷淋设施整体检查')
  AND (photo_required <> 'required' OR requirement LIKE '整组%');

-- +goose Down

UPDATE public.check_template_item SET
  photo_required = 'none',
  requirement = '整组拍 1 张照片，逐项核对观察点；异常观察点在 App 上勾选上报'
WHERE name IN ('消火栓箱整体检查', '灭火器整体检查', '疏散设施整体检查', '报警喷淋设施整体检查');
