package service

import (
	"github.com/gin-gonic/gin"

	"anxuncloud/internal/module/inspection/model"
	"anxuncloud/internal/pkg/timefmt"
	"anxuncloud/internal/pkg/uploadfile"

	"gorm.io/gorm"
)

// RecordFlatPhotos 聚合一条打卡的全部逐项照片为扁平视图
// [{item, file_id, url, watermarked_url?, exif_time?}]（按逐项 sort + 照片顺序）。
// 照片唯一归属逐项，item.photos 仅存 upload_file.id。
func RecordFlatPhotos(db *gorm.DB, recordID string) []gin.H {
	var items []model.CheckinRecordItem
	db.Select("name", "photos").Where("record_id = ?", recordID).Order("sort").Find(&items)
	refs := make([]string, 0, 8)
	for _, it := range items {
		refs = append(refs, it.Photos...)
	}
	files := uploadfile.ByIDs(db, refs)
	out := make([]gin.H, 0, len(refs))
	for _, it := range items {
		for _, ref := range it.Photos {
			f := files[ref]
			entry := gin.H{"item": it.Name, "file_id": f.ID, "url": f.URL}
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

// ItemPhotoURLs 逐项照片 file_id → 可访问 URL（优先水印图）。
func ItemPhotoURLs(db *gorm.DB, refs []string) []string {
	urls := make([]string, 0, len(refs))
	if len(refs) == 0 {
		return urls
	}
	files := uploadfile.ByIDs(db, refs)
	for _, ref := range refs {
		f := files[ref]
		if f.WatermarkedURL != "" {
			urls = append(urls, f.WatermarkedURL)
		} else {
			urls = append(urls, f.URL)
		}
	}
	return urls
}
