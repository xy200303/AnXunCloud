package service

import (
	"github.com/gin-gonic/gin"

	"anxuncloud/internal/middleware"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/errs"
)

func (s *StaffService) FlowPreview(c *gin.Context, communityID, flowCode string) (gin.H, *errs.Error) {
	if be := middleware.CheckCommunity(s.db, c, communityID); be != nil {
		return nil, be
	}
	if !sysmodel.ValidApprovalFlowCode(flowCode) {
		return nil, errs.ErrParam.WithMsg("flow_code 取值非法")
	}
	flow, source := ResolveFlowWithSource(s.db, communityID, flowCode)
	steps := make([]gin.H, 0, len(flow))
	for i, st := range flow {
		if st.Kind == sysmodel.FlowStepKindAI {
			steps = append(steps, gin.H{"index": i, "name": st.Name, "kind": "ai"})
			continue
		}
		voters := s.userBriefs(RoleCandidateUserIDsForCommunity(s.db, communityID, st.RoleCode))
		emptyReason := ""
		if len(voters) == 0 {
			emptyReason = "no_member"
		}
		steps = append(steps, gin.H{
			"index": i, "name": st.Name, "mode": st.Mode, "role_code": st.RoleCode,
			"voters": voters, "voter_source": source, "empty_reason": emptyReason,
		})
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
