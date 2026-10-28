package service

import (
	"fmt"
	"strings"

	"anxuncloud/internal/module/acceptance/model"
	"anxuncloud/internal/pkg/ooxml"
)

func (s *BatchService) fillCoveredNormal(mother *ooxml.Template, targets []model.Target, records []model.Record, names map[string]string, category string) error {
	for _, target := range targets {
		if acceptanceCategoryAliases[target.CategoryCode] != category || !target.CoverageConfirmed {
			continue
		}
		targetRecords := []model.Record{}
		for _, record := range records {
			if record.TargetID == target.ID {
				targetRecords = append(targetRecords, record)
			}
		}
		for _, tableCode := range target.TableCodes {
			spec, ok := acceptanceTableSlots[tableCode]
			if !ok {
				return fmt.Errorf("覆盖专项没有母版槽位: %s", tableCode)
			}
			if spec.kind != "matrix" {
				continue
			}
			if err := s.fillCoveredTableNormal(mother, spec, tableCode, targetRecords, names); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *BatchService) fillCoveredTableNormal(mother *ooxml.Template, spec acceptanceTableSpec, tableCode string, records []model.Record, names map[string]string) error {
	existing := map[string]bool{}
	for _, record := range records {
		existing[record.TableCode+"\x00"+record.ItemCode] = true
	}
	for key, itemName := range names {
		if !strings.HasPrefix(key, tableCode+"\x00") || existing[key] {
			continue
		}
		_, itemCode, _ := strings.Cut(key, "\x00")
		record := model.Record{TableCode: tableCode, ItemCode: itemCode, Result: "normal"}
		if err := s.fillAcceptanceRecord(mother, spec, record, names); err != nil {
			return fmt.Errorf("写入覆盖合格项 %s 失败: %w", itemName, err)
		}
	}
	return nil
}
