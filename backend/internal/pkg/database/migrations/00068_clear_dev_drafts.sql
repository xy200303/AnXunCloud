-- 00068：开发阶段数据清理——清空逐项过程草稿与点位凭证草稿。
-- 00063-00067 改了草稿语义（draft_kind/shoot_*/凭证草稿），存量测试草稿全部作废；
-- 草稿只是过程数据（不影响正式打卡记录），清空无任何业务损失。幂等可重复执行。

-- +goose Up

DELETE FROM checkin_item_draft;
DELETE FROM checkin_point_cred_draft;

-- +goose Down

-- 数据清理不可回滚（草稿为过程数据，无需回补）
