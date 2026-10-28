package ooxml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"
)

const namespaceW = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"

func xmlName(local string) xml.Name {
	return xml.Name{Space: namespaceW, Local: local}
}

func (t *Template) SetCellImage(tableIndex, rowIndex, columnIndex int, data []byte, imageType string, heightEMU int64, appendMode ...bool) error {
	table, err := t.table(tableIndex)
	if err != nil {
		return err
	}
	rows := children(table, "tr")
	if rowIndex < 0 || rowIndex >= len(rows) {
		return fmt.Errorf("图片行索引无效")
	}
	cells := children(rows[rowIndex], "tc")
	if columnIndex < 0 || columnIndex >= len(cells) {
		return fmt.Errorf("图片列索引无效")
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return fmt.Errorf("图片无法解码")
	}
	extension := ""
	mimeType := ""
	switch strings.ToLower(imageType) {
	case "jpg", "jpeg", "image/jpeg", "image/jpg":
		extension, mimeType = "jpeg", "image/jpeg"
	case "png", "image/png":
		extension, mimeType = "png", "image/png"
	default:
		return fmt.Errorf("仅支持 JPEG/PNG 图片")
	}
	if heightEMU <= 0 {
		heightEMU = 360000
	}
	if heightEMU > 540000 {
		heightEMU = 540000
	}
	widthEMU := heightEMU * int64(config.Width) / int64(config.Height)
	if widthEMU > 1800000 {
		widthEMU = 1800000
	}
	number := t.nextImg
	t.nextImg++
	name := fmt.Sprintf("mother-image-%d.%s", number, extension)
	relationshipID := fmt.Sprintf("rIdMotherImage%d", number)
	t.media[name] = append([]byte(nil), data...)
	t.rels.Children = append(t.rels.Children, &xmlNode{
		Name: xml.Name{Local: "Relationship"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "Id"}, Value: relationshipID},
			{Name: xml.Name{Local: "Type"}, Value: "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image"},
			{Name: xml.Name{Local: "Target"}, Value: "media/" + name},
		},
	})
	t.ensureImageType(extension, mimeType)
	drawing := fmt.Sprintf(`<w:drawing xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture"><wp:inline distT="0" distB="0" distL="0" distR="0"><wp:extent cx="%d" cy="%d"/><wp:effectExtent l="0" t="0" r="0" b="0"/><wp:docPr id="%d" name="图片 %d"/><wp:cNvGraphicFramePr><a:graphicFrameLocks noChangeAspect="1"/></wp:cNvGraphicFramePr><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic><pic:nvPicPr><pic:cNvPr id="%d" name="%s"/><pic:cNvPicPr/></pic:nvPicPr><pic:blipFill><a:blip r:embed="%s"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill><pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr></pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing>`, widthEMU, heightEMU, number+1000, number+1000, number+1000, name, relationshipID, widthEMU, heightEMU)
	node, err := parseXML([]byte(drawing))
	if err != nil {
		return err
	}
	paragraph := child(cells[columnIndex], "p")
	if paragraph == nil {
		paragraph = &xmlNode{Name: xmlName("p")}
		cells[columnIndex].Children = append(cells[columnIndex].Children, paragraph)
	}
	properties := child(paragraph, "pPr")
	if len(appendMode) == 0 || !appendMode[0] {
		paragraph.Children = nil
	}
	if properties != nil && (len(appendMode) == 0 || !appendMode[0]) {
		paragraph.Children = append(paragraph.Children, properties)
	}
	run := &xmlNode{Name: xmlName("r")}
	run.Children = append(run.Children, node)
	paragraph.Children = append(paragraph.Children, run)
	return nil
}

func (t *Template) ensureImageType(extension, mimeType string) {
	for _, node := range t.types.Children {
		if node.Name.Local == "Default" && strings.EqualFold(attr(node, "Extension"), extension) {
			return
		}
	}
	t.types.Children = append(t.types.Children, &xmlNode{
		Name: xml.Name{Local: "Default"},
		Attr: []xml.Attr{{Name: xml.Name{Local: "Extension"}, Value: extension}, {Name: xml.Name{Local: "ContentType"}, Value: mimeType}},
	})
}
