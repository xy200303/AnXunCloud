package service

import (
	"fmt"

	"anxuncloud/internal/module/acceptance/model"
	"anxuncloud/internal/pkg/ooxml"
	"anxuncloud/internal/pkg/uploadfile"
)

func (s *BatchService) fillAcceptanceDocumentPhotos(mother *ooxml.Template, spec acceptanceTableSpec, records []model.Record, names map[string]string) error {
	rows, err := mother.TableRowCount(spec.photo)
	if err != nil {
		return err
	}
	if err := mother.ResizeRows(spec.photo, 1, rows, (len(records)+1)/2); err != nil {
		return err
	}
	for i, record := range records {
		row, nameColumn, imageColumn, sequenceColumn := 1+i/2, 1, 2, 0
		if i%2 == 1 {
			nameColumn, imageColumn, sequenceColumn = 5, 6, 3
		}
		if err := mother.SetCellText(spec.photo, row, sequenceColumn, fmt.Sprint(i+1), 0); err != nil {
			return err
		}
		name := names[record.TableCode+"\x00"+record.ItemCode]
		if record.Note != "" {
			name += "：" + record.Note
		}
		if err := mother.SetCellText(spec.photo, row, nameColumn, name, 0); err != nil {
			return err
		}
		for _, fileID := range record.Photos {
			file, err := uploadfile.ByID(s.db, fileID)
			if err != nil {
				return err
			}
			data, err := s.store.ReadFile(file.StorageKey)
			if err != nil {
				return err
			}
			if err := mother.SetCellImage(spec.photo, row, imageColumn, data, file.MimeType, 288000, true); err != nil {
				return err
			}
		}
	}
	return nil
}
