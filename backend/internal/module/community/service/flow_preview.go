// 审批链配置预览：把「每个环节到底谁审」显性化给配置页（根治方案 P2）。
// 名单来源与空态原因三态（unconfigured/no_member/skipped）与线上走链同口径：
// 槽位→绑定岗位（项目→租户→平台回落）→编制在职成员；空名单环节走链时自动跳过。
package service

import (
	"github.com/gin-gonic/gin"

	"anxuncloud/internal/middleware"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
)

// FlowPreview 审批链预览：链来源 + 逐环节名单/来源/空态原因。
// 报告链巡检员确认环节候选人 = 当月任务巡检员（生成时定），空则编制巡检员（巡查执行槽位），预览按编制兜底口径展示。
func (s *StaffService) FlowPreview(c *gin.Context, communityID, flowCode string) (gin.H, *errs.Error) {
	if be := middleware.CheckCommunity(s.db, c, communityID); be != nil {
		return nil, be
	}
	switch flowCode {
	case sysmodel.FlowCheckinReview, sysmodel.FlowMaintReview, sysmodel.FlowReportReview:
	default:
		return nil, errs.ErrParam.WithMsg("flow_code 取值非法")
	}
	flow, source := ResolveFlowWithSource(s.db, communityID, flowCode)
	steps := make([]gin.H, 0, len(flow))
	for i, st := range flow {
		if st.Kind == sysmodel.FlowStepKindAI {
			steps = append(steps, gin.H{"index": i, "name": st.Name, "kind": "ai"})
			continue
		}
		slot := st.Slot
		// 报告巡检员确认环节：名单不走槽位绑定，预览展示编制巡检员兜底口径
		fallbackInspector := flowCode == sysmodel.FlowReportReview && slot == sysmodel.SlotReportInspector
		voterSlot := slot
		if fallbackInspector {
			voterSlot = sysmodel.SlotPatrolExecute
		}
		posts, postSource := resolveSlotPosts(s.db, communityID, voterSlot)
		userIDs := SlotUserIDs(s.db, communityID, voterSlot)
		voters := s.userBriefs(userIDs)
		emptyReason := ""
		if len(voters) == 0 {
			switch {
			case postSource == "":
				emptyReason = "unconfigured" // 无任何绑定
			case len(posts) == 0:
				emptyReason = "skipped" // 岗位留空 = 显式跳过
			default:
				emptyReason = "no_member" // 有绑定但项目内无在职成员
			}
		}
		item := gin.H{
			"index": i, "name": st.Name, "mode": st.Mode, "slot": slot,
			"voters": voters, "voter_source": postSource, "empty_reason": emptyReason,
		}
		if fallbackInspector {
			item["voter_note"] = "生成报告时按当月任务巡检员确定；无任务巡检员时回落本名单（编制内巡检员）"
		}
		steps = append(steps, item)
	}
	return gin.H{"source": source, "steps": steps}, nil
}

// userBriefs 用户 id → {id,name}（保持传入顺序）。
func (s *StaffService) userBriefs(ids []string) []gin.H {
	out := make([]gin.H, 0, len(ids))
	if len(ids) == 0 {
		return out
	}
	names := map[string]string{}
	var users []sysmodel.SysUser
	s.db.Select("id", "name").Where("id IN ?", ids).Find(&users)
	for _, u := range users {
		names[u.ID] = u.Name
	}
	for _, id := range ids {
		out = append(out, gin.H{"id": id, "name": names[id]})
	}
	return out
}
