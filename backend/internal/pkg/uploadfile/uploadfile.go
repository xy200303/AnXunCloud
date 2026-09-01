package uploadfile

import (
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	sysmodel "anxuncloud/internal/module/system/model"
)

// ByID resolves an upload_file by its canonical UUID.
func ByID(db *gorm.DB, id string) (sysmodel.UploadFile, error) {
	var f sysmodel.UploadFile
	id = strings.TrimSpace(id)
	if id == "" {
		return f, gorm.ErrRecordNotFound
	}
	if _, err := uuid.Parse(id); err != nil {
		return f, gorm.ErrRecordNotFound
	}
	err := db.Where("id = ?", id).First(&f).Error
	return f, err
}

// ByIDs resolves multiple upload_file ids in one query, returning an id → row lookup map.
func ByIDs(db *gorm.DB, ids []string) map[string]sysmodel.UploadFile {
	out := make(map[string]sysmodel.UploadFile, len(ids))
	valid := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, err := uuid.Parse(id); err != nil {
			continue
		}
		valid = append(valid, id)
	}
	if len(valid) == 0 {
		return out
	}
	var rows []sysmodel.UploadFile
	db.Where("id IN ?", valid).Find(&rows)
	for i := range rows {
		out[rows[i].ID] = rows[i]
	}
	return out
}
