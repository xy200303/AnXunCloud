package service

import (
	"github.com/gin-gonic/gin"

	"anxuncloud/internal/module/inspection/model"
	sysmodel "anxuncloud/internal/module/system/model"
	"anxuncloud/internal/pkg/timefmt"

	"gorm.io/gorm"
)

// RecordFlatPhotos 聚合一条打卡的全部逐项照片为扁平视图
// [{item, file_key, url, watermarked_url?, exif_time?}]（按逐项 sort + 照片顺序）。
// 照片唯一归属逐项（v21 起无记录级照片）；url/水印/EXIF 等元信息从 upload_file 派生。
func RecordFlatPhotos(db *gorm.DB, recordID string) []gin.H {
	var items []model.CheckinRecordItem
	db.Select("name", "photos").Where("record_id = ?", recordID).Order("sort").Find(&items)
	keys := make([]string, 0, 8)
	for _, it := range items {
		keys = append(keys, it.Photos...)
	}
	files := map[string]sysmodel.UploadFile{}
	if len(keys) > 0 {
		var rows []sysmodel.UploadFile
		db.Where("file_key IN ?", keys).Find(&rows)
		for _, f := range rows {
			files[f.FileKey] = f
		}
	}
	out := make([]gin.H, 0, len(keys))
	for _, it := range items {
		for _, k := range it.Photos {
			f := files[k]
			entry := gin.H{"item": it.Name, "file_key": k, "url": f.URL}
			if f.WatermarkedURL != "" {
				entry["watermarked_url"] = f.WatermarkedURL
			}
			if f.ExifTime != nil {
				entry["exif_time"] = timefmt.T(*f.ExifTime)
			}
			out = append(out, entry)
		}
	}
	return out
}

// RecordPhotoCount 一条打卡的照片总数（逐项照片聚合；列表页 photo_count 用）。
func RecordPhotoCount(db *gorm.DB, recordID string) int {
	var items []model.CheckinRecordItem
	db.Select("photos").Where("record_id = ?", recordID).Find(&items)
	n := 0
	for _, it := range items {
		n += len(it.Photos)
	}
	return n
}

// ItemPhotoURLs 逐项照片 file_key → 可访问 URL（优先水印图）。
func ItemPhotoURLs(db *gorm.DB, keys []string) []string {
	urls := make([]string, 0, len(keys))
	if len(keys) == 0 {
		return urls
	}
	var rows []sysmodel.UploadFile
	db.Where("file_key IN ?", keys).Find(&rows)
	byKey := map[string]sysmodel.UploadFile{}
	for _, f := range rows {
		byKey[f.FileKey] = f
	}
	for _, k := range keys {
		f := byKey[k]
		if f.WatermarkedURL != "" {
			urls = append(urls, f.WatermarkedURL)
		} else {
			urls = append(urls, f.URL)
		}
	}
	return urls
}
