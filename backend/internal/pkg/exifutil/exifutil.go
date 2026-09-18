// Package exifutil 解析 JPEG EXIF 拍摄时间（用于防作弊校验）。
package exifutil

import (
	"bytes"
	"io"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

// ReadShotTimeBytes 从字节读取 JPEG 拍摄时间（云驱动下文件不落本地盘时用）。
func ReadShotTimeBytes(data []byte) *time.Time {
	return readShotTime(bytes.NewReader(data))
}

func readShotTime(r io.Reader) *time.Time {
	x, err := exif.Decode(r)
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
