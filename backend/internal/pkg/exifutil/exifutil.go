// Package exifutil 解析 JPEG EXIF 拍摄时间（用于防作弊校验）。
package exifutil

import (
	"os"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

// ReadShotTime 读取 JPEG 的拍摄时间（DateTimeOriginal 优先）；失败返回 nil（不阻塞主流程）。
func ReadShotTime(path string) *time.Time {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	x, err := exif.Decode(f)
	if err != nil {
		return nil
	}
	// DateTime() 依次尝试 DateTimeOriginal / DateTimeDigitized / DateTime
	tm, err := x.DateTime()
	if err != nil {
		return nil
	}
	// EXIF 时间无时区，按本地时区解释
	local := time.Date(tm.Year(), tm.Month(), tm.Day(), tm.Hour(), tm.Minute(), tm.Second(), 0, time.Local)
	return &local
}
