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

// ByRef resolves an upload_file by canonical id or legacy file_key for non-inspection historical data.
func ByRef(db *gorm.DB, ref string) (sysmodel.UploadFile, error) {
	if f, err := ByID(db, ref); err == nil {
		return f, nil
	}
	var f sysmodel.UploadFile
	err := db.Where("storage_key = ?", strings.TrimSpace(ref)).First(&f).Error
	return f, err
}

func ByIDs(db *gorm.DB, ids []string) map[string]sysmodel.UploadFile {
	out := map[string]sysmodel.UploadFile{}
	if len(ids) == 0 { return out }
	var rows []sysmodel.UploadFile
	db.Where("id IN ?", ids).Find(&rows)
	for _, f := range rows { out[f.ID] = f }
	return out
}

// ByRefs resolves multiple refs in one query and returns a lookup map keyed by both id and file_key.
func ByRefs(db *gorm.DB, refs []string) map[string]sysmodel.UploadFile {
	out := map[string]sysmodel.UploadFile{}
	if len(refs) == 0 {
		return out
	}
	ids := make([]string, 0, len(refs))
	keys := make([]string, 0, len(refs))
	seen := map[string]bool{}
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" || seen[ref] {
			continue
		}
		seen[ref] = true
		if _, err := uuid.Parse(ref); err == nil {
			ids = append(ids, ref)
		} else {
			keys = append(keys, ref)
		}
	}
	if len(ids) > 0 {
		var rows []sysmodel.UploadFile
		db.Where("id IN ?", ids).Find(&rows)
		for _, f := range rows {
			out[f.ID] = f
			out[f.StorageKey] = f
		}
	}
	if len(keys) > 0 {
		var rows []sysmodel.UploadFile
		db.Where("storage_key IN ?", keys).Find(&rows)
		for _, f := range rows {
			out[f.ID] = f
			out[f.StorageKey] = f
		}
	}
	return out
}
