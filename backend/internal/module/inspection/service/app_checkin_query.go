package service

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	insmodel "anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/pkg/errs"
	"anxuncloud/internal/pkg/strutil"
	"anxuncloud/internal/pkg/timefmt"
)

type AppCheckinQueryService struct{ db *gorm.DB }

func NewAppCheckinQueryService(db *gorm.DB) *AppCheckinQueryService {
	return &AppCheckinQueryService{db: db}
}

func (s *AppCheckinQueryService) CheckinItems(inspectorID, checkinID string) ([]gin.H, *errs.Error) {
	var rec insmodel.CheckinRecord
	if err := s.db.Select("id", "inspector_id").First(&rec, "id = ?", checkinID).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if rec.InspectorID != inspectorID {
		return nil, errs.ErrNotFound.WithMsg("打卡记录不存在或不属于当前巡检员")
	}
	var items []insmodel.CheckinRecordItem
	if err := s.db.Where("record_id = ?", rec.ID).Order("sort ASC").Find(&items).Error; err != nil {
		return nil, errs.ErrInternal
	}
	out := make([]gin.H, 0, len(items))
	for i := range items {
		it := &items[i]
		out = append(out, gin.H{
			"name": it.Name, "result": it.Result,
			"template_id": it.TemplateID, "item_code": it.ItemCode, "equipment_id": it.EquipmentID,
			"standard": it.Standard, "value_num": it.ValueNum, "value_date": it.ValueDate, "value_text": it.ValueText,
			"ai_verdict": strutil.StrVal(it.AIVerdict), "ai_reason": strutil.StrVal(it.AIReason),
			"ai_reading":     strutil.StrVal(it.AIReading),
			"note":           it.Note,
			"exception_type": it.ExceptionType,
			"abnormal_tags":  it.AbnormalTags,
			"photo_urls":     ItemPhotoURLs(s.db, it.Photos),
			"shoot_lng":      it.ShootLng, "shoot_lat": it.ShootLat, "shoot_at": timefmt.TP(it.ShootAt),
			"suspicious": it.Suspicious, "suspicious_reason": it.SuspiciousReason,
		})
	}
	return out, nil
}

func (s *AppCheckinQueryService) CheckinBrief(inspectorID, checkinID string) (gin.H, *errs.Error) {
	var rec insmodel.CheckinRecord
	if err := s.db.Select("id", "inspector_id", "task_id", "point_id", "audit_status").First(&rec, "id = ?", checkinID).Error; err != nil {
		return nil, errs.ErrNotFound
	}
	if rec.InspectorID != inspectorID {
		return nil, errs.ErrNotFound.WithMsg("打卡记录不存在或不属于当前巡检员")
	}
	return gin.H{
		"id": rec.ID, "task_id": rec.TaskID, "point_id": rec.PointID,
		"audit_status": rec.AuditStatus,
	}, nil
}
