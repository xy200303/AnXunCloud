package ooxml

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
)

type Template struct {
	parts   map[string][]byte
	doc     *xmlNode
	rels    *xmlNode
	types   *xmlNode
	media   map[string][]byte
	nextImg int
}

func OpenTemplate(data []byte) (*Template, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("母版不是有效 DOCX: %w", err)
	}
	if len(reader.File) == 0 || len(reader.File) > 1000 {
		return nil, fmt.Errorf("母版部件数量异常")
	}
	template := &Template{parts: map[string][]byte{}, media: map[string][]byte{}, nextImg: 1}
	var total int64
	for _, file := range reader.File {
		if strings.Contains(file.Name, "..") || strings.HasPrefix(file.Name, "/") || strings.Contains(file.Name, "\\") {
			return nil, fmt.Errorf("母版包含非法路径")
		}
		total += int64(file.UncompressedSize64)
		if total > 100<<20 || file.UncompressedSize64 > 20<<20 {
			return nil, fmt.Errorf("母版解压大小超过限制")
		}
		content, err := file.Open()
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(io.LimitReader(content, 20<<20+1))
		_ = content.Close()
		if err != nil {
			return nil, err
		}
		lower := strings.ToLower(string(body))
		for _, forbidden := range []string{"<!doctype", "vbaproject", "oleobject", "activex", `targetmode="external"`} {
			if strings.Contains(lower, forbidden) {
				return nil, fmt.Errorf("母版包含禁止内容: %s", forbidden)
			}
		}
		template.parts[file.Name] = body
	}
	for _, name := range []string{"word/document.xml", "word/_rels/document.xml.rels", "[Content_Types].xml"} {
		if len(template.parts[name]) == 0 {
			return nil, fmt.Errorf("母版缺少必要部件: %s", name)
		}
	}
	if template.doc, err = parseXML(template.parts["word/document.xml"]); err != nil {
		return nil, fmt.Errorf("母版 document.xml 无效: %w", err)
	}
	if template.rels, err = parseXML(template.parts["word/_rels/document.xml.rels"]); err != nil {
		return nil, fmt.Errorf("母版关系文件无效: %w", err)
	}
	if template.types, err = parseXML(template.parts["[Content_Types].xml"]); err != nil {
		return nil, fmt.Errorf("母版内容类型无效: %w", err)
	}
	return template, nil
}

func (t *Template) table(index int) (*xmlNode, error) {
	tables := descendants(t.doc, "tbl")
	if index < 0 || index >= len(tables) {
		return nil, fmt.Errorf("母版表格索引 %d 超出数量 %d", index, len(tables))
	}
	return tables[index], nil
}

func (t *Template) CellText(tableIndex, rowIndex, columnIndex int) (string, error) {
	table, err := t.table(tableIndex)
	if err != nil {
		return "", err
	}
	rows := children(table, "tr")
	if rowIndex < 0 || rowIndex >= len(rows) {
		return "", fmt.Errorf("母版表格 %d 行索引 %d 无效", tableIndex, rowIndex)
	}
	cells := children(rows[rowIndex], "tc")
	if columnIndex < 0 || columnIndex >= len(cells) {
		return "", fmt.Errorf("母版表格 %d 列索引 %d 无效", tableIndex, columnIndex)
	}
	var text strings.Builder
	for _, node := range descendants(cells[columnIndex], "t") {
		text.WriteString(node.Text)
	}
	return strings.TrimSpace(text.String()), nil
}

func NormalizeCellText(text string) string {
	return strings.Join(strings.FieldsFunc(text, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '/' || r == '／' || r == 0xa0
	}), "")
}

func (t *Template) FindCell(tableIndex int, text string) (int, int, bool, error) {
	table, err := t.table(tableIndex)
	if err != nil {
		return 0, 0, false, err
	}
	expected := NormalizeCellText(text)
	for rowIndex, row := range children(table, "tr") {
		for columnIndex, cell := range children(row, "tc") {
			var content strings.Builder
			for _, node := range descendants(cell, "t") {
				content.WriteString(node.Text)
			}
			if NormalizeCellText(content.String()) == expected {
				return rowIndex, columnIndex, true, nil
			}
		}
	}
	return 0, 0, false, nil
}

func (t *Template) TableRowCount(tableIndex int) (int, error) {
	table, err := t.table(tableIndex)
	if err != nil {
		return 0, err
	}
	return len(children(table, "tr")), nil
}

func (t *Template) SetCellText(tableIndex, rowIndex, columnIndex int, text string, fontHalfPoint int) error {
	table, err := t.table(tableIndex)
	if err != nil {
		return err
	}
	rows := children(table, "tr")
	if rowIndex < 0 || rowIndex >= len(rows) {
		return fmt.Errorf("母版表格 %d 行索引 %d 无效", tableIndex, rowIndex)
	}
	cells := children(rows[rowIndex], "tc")
	if columnIndex < 0 || columnIndex >= len(cells) {
		return fmt.Errorf("母版表格 %d 行 %d 列索引 %d 无效", tableIndex, rowIndex, columnIndex)
	}
	setCellText(cells[columnIndex], text, fontHalfPoint)
	return nil
}

func (t *Template) SetCellTextLast(tableIndex, rowIndex int, text string, fontHalfPoint int) error {
	table, err := t.table(tableIndex)
	if err != nil {
		return err
	}
	rows := children(table, "tr")
	if rowIndex < 0 || rowIndex >= len(rows) {
		return fmt.Errorf("母版表格 %d 行索引 %d 无效", tableIndex, rowIndex)
	}
	cells := children(rows[rowIndex], "tc")
	if len(cells) == 0 {
		return fmt.Errorf("母版表格 %d 行 %d 没有单元格", tableIndex, rowIndex)
	}
	setCellText(cells[len(cells)-1], text, fontHalfPoint)
	return nil
}

func setCellText(cell *xmlNode, text string, fontHalfPoint int) {
	properties := child(cell, "tcPr")
	if properties == nil {
		properties = &xmlNode{Name: xmlName("tcPr")}
		cell.Children = append([]*xmlNode{properties}, cell.Children...)
	}
	if child(properties, "noWrap") == nil {
		properties.Children = append(properties.Children, &xmlNode{Name: xmlName("noWrap")})
	}
	paragraph := child(cell, "p")
	if paragraph == nil {
		paragraph = &xmlNode{Name: xmlName("p")}
		cell.Children = append(cell.Children, paragraph)
	}
	setParagraphText(paragraph, text, fontHalfPoint)
}

func setParagraphText(paragraph *xmlNode, text string, fontHalfPoint int) {
	paragraphProperties := child(paragraph, "pPr")
	var runProperties *xmlNode
	for _, run := range children(paragraph, "r") {
		if properties := child(run, "rPr"); properties != nil {
			runProperties = properties
			break
		}
	}
	paragraph.Children = nil
	if paragraphProperties != nil {
		paragraph.Children = append(paragraph.Children, paragraphProperties)
	}
	run := &xmlNode{Name: xmlName("r")}
	if runProperties != nil {
		runProperties = runProperties.clone()
		if fontHalfPoint > 0 {
			for _, name := range []string{"sz", "szCs"} {
				properties := child(runProperties, name)
				if properties == nil {
					properties = &xmlNode{Name: xmlName(name)}
					runProperties.Children = append(runProperties.Children, properties)
				}
				setAttr(properties, namespaceW, "val", fmt.Sprint(fontHalfPoint))
			}
		}
		run.Children = append(run.Children, runProperties)
	}
	textNode := &xmlNode{Name: xmlName("t"), Text: text}
	setAttr(textNode, "xml", "space", "preserve")
	run.Children = append(run.Children, textNode)
	paragraph.Children = append(paragraph.Children, run)
}

func (t *Template) SetParagraphText(prefix, text string) error {
	for _, paragraph := range descendants(t.doc, "p") {
		var content strings.Builder
		for _, textNode := range descendants(paragraph, "t") {
			content.WriteString(textNode.Text)
		}
		if strings.HasPrefix(strings.TrimSpace(content.String()), prefix) {
			setParagraphText(paragraph, text, 0)
			return nil
		}
	}
	return fmt.Errorf("母版未找到段落: %s", prefix)
}

func (t *Template) ResizeRows(tableIndex, firstRow, endExclusive, count int) error {
	table, err := t.table(tableIndex)
	if err != nil {
		return err
	}
	rows := children(table, "tr")
	if firstRow < 0 || firstRow >= endExclusive || endExclusive > len(rows) || count < 0 {
		return fmt.Errorf("母版行动态范围无效")
	}
	childrenOut := []*xmlNode{}
	rowIndex := -1
	inserted := false
	for _, current := range table.Children {
		if current.Name.Local == "tr" {
			rowIndex++
		}
		if current.Name.Local == "tr" && rowIndex >= firstRow && rowIndex < endExclusive {
			if !inserted {
				for i := 0; i < count; i++ {
					childrenOut = append(childrenOut, rows[firstRow].clone())
				}
				inserted = true
			}
			continue
		}
		childrenOut = append(childrenOut, current)
	}
	table.Children = childrenOut
	return nil
}

func (t *Template) SetRecordRows(tableIndex, templateRow int, keepLast bool, values []map[int]string, fontHalfPoint int) error {
	if len(values) == 0 {
		return nil
	}
	table, err := t.table(tableIndex)
	if err != nil {
		return err
	}
	rows := children(table, "tr")
	if templateRow < 0 || templateRow >= len(rows) {
		return fmt.Errorf("母版记录行索引无效")
	}
	end := len(rows)
	if keepLast {
		end--
	}
	if templateRow >= end {
		return fmt.Errorf("母版记录行范围无效")
	}
	newChildren := []*xmlNode{}
	inserted := false
	rowIndex := -1
	for _, current := range table.Children {
		if current.Name.Local == "tr" {
			rowIndex++
		}
		if current.Name.Local == "tr" && rowIndex >= templateRow && rowIndex < end {
			if !inserted {
				for _, value := range values {
					clone := rows[templateRow].clone()
					for column, text := range value {
						cells := children(clone, "tc")
						if column < 0 || column >= len(cells) {
							return fmt.Errorf("记录列索引 %d 无效", column)
						}
						setCellText(cells[column], text, fontHalfPoint)
					}
					newChildren = append(newChildren, clone)
				}
				inserted = true
			}
			continue
		}
		newChildren = append(newChildren, current)
	}
	table.Children = newChildren
	return nil
}

func (t *Template) ReplaceTableSections(pairs [][2]int, count int) (int, error) {
	if len(pairs) == 0 || count < 1 {
		return 0, fmt.Errorf("母版重复段参数无效")
	}
	body := child(t.doc, "body")
	if body == nil {
		return 0, fmt.Errorf("母版缺少文档主体")
	}
	type section struct{ start, end int }
	sections := make([]section, 0, len(pairs))
	tableIndex := -1
	pairIndex := 0
	for i, node := range body.Children {
		if node.Name.Local != "tbl" {
			continue
		}
		tableIndex++
		if pairIndex >= len(pairs) || tableIndex != pairs[pairIndex][0] {
			continue
		}
		if i == 0 || body.Children[i-1].Name.Local != "p" {
			return 0, fmt.Errorf("母版重复段缺少标题")
		}
		sections = append(sections, section{start: i - 1})
		pairIndex++
	}
	if pairIndex != len(pairs) {
		return 0, fmt.Errorf("母版重复段表格索引不匹配")
	}
	tableIndex = -1
	photoIndex := 0
	for i, node := range body.Children {
		if node.Name.Local != "tbl" {
			continue
		}
		tableIndex++
		if photoIndex >= len(pairs) || tableIndex != pairs[photoIndex][1] {
			continue
		}
		end := i
		if end+1 < len(body.Children) && body.Children[end+1].Name.Local == "p" {
			end++
		}
		sections[photoIndex].end = end
		photoIndex++
	}
	if photoIndex != len(pairs) {
		return 0, fmt.Errorf("母版照片段表格索引不匹配")
	}
	for i := 1; i < len(sections); i++ {
		if sections[i].start <= sections[i-1].end {
			return 0, fmt.Errorf("母版重复段范围重叠")
		}
	}
	source := body.Children[sections[0].start : sections[0].end+1]
	out := []*xmlNode{}
	for i, node := range body.Children {
		if i == sections[0].start {
			for copyIndex := 0; copyIndex < count; copyIndex++ {
				for _, sourceNode := range source {
					out = append(out, sourceNode.clone())
				}
			}
		}
		skip := false
		for _, current := range sections {
			if i >= current.start && i <= current.end {
				skip = true
				break
			}
		}
		if !skip {
			out = append(out, node)
		}
	}
	body.Children = out
	return pairs[0][0] + count*2, nil
}

func (t *Template) Build() ([]byte, error) {
	var document, relationships, contentTypes bytes.Buffer
	writeXMLNode(&document, t.doc)
	writeXMLNode(&relationships, t.rels)
	writeXMLNode(&contentTypes, t.types)
	t.parts["word/document.xml"] = document.Bytes()
	t.parts["word/_rels/document.xml.rels"] = relationships.Bytes()
	t.parts["[Content_Types].xml"] = contentTypes.Bytes()
	for name, data := range t.media {
		t.parts["word/media/"+name] = data
	}
	names := make([]string, 0, len(t.parts))
	for name := range t.parts {
		names = append(names, name)
	}
	sort.Strings(names)
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, name := range names {
		part, err := writer.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(t.parts[name]); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
