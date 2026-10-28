package service

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"

	"anxuncloud/internal/module/acceptance/model"
)

func acceptanceFilesDigest(files []model.ReportFile) string {
	ordered := append([]model.ReportFile(nil), files...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].VolumeCode != ordered[j].VolumeCode {
			return ordered[i].VolumeCode < ordered[j].VolumeCode
		}
		return ordered[i].Format < ordered[j].Format
	})
	digest := sha256.New()
	for _, file := range ordered {
		digest.Write([]byte(file.VolumeCode))
		digest.Write([]byte{0})
		digest.Write([]byte(file.Format))
		digest.Write([]byte{0})
		digest.Write([]byte(file.SHA256))
		digest.Write([]byte{0})
	}
	return hex.EncodeToString(digest.Sum(nil))
}
