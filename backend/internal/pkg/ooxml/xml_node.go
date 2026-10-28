package ooxml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type xmlNode struct {
	Name     xml.Name
	Attr     []xml.Attr
	Children []*xmlNode
	Text     string
}

func parseXML(data []byte) (*xmlNode, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var root *xmlNode
	stack := []*xmlNode{}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch value := token.(type) {
		case xml.StartElement:
			node := &xmlNode{Name: value.Name, Attr: append([]xml.Attr(nil), value.Attr...)}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, node)
			} else if root != nil {
				return nil, fmt.Errorf("XML 存在多个根节点")
			} else {
				root = node
			}
			stack = append(stack, node)
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, fmt.Errorf("XML 结束标签不匹配")
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].Text += string(value)
			}
		}
	}
	if root == nil || len(stack) != 0 {
		return nil, fmt.Errorf("XML 结构不完整")
	}
	return root, nil
}

var namespacePrefixes = map[string]string{
	"http://schemas.openxmlformats.org/wordprocessingml/2006/main":           "w",
	"http://schemas.openxmlformats.org/officeDocument/2006/relationships":    "r",
	"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing": "wp",
	"http://schemas.openxmlformats.org/drawingml/2006/main":                  "a",
	"http://schemas.openxmlformats.org/drawingml/2006/picture":               "pic",
	"http://schemas.openxmlformats.org/markup-compatibility/2006":            "mc",
	"http://schemas.microsoft.com/office/word/2010/wordml":                   "w14",
	"http://schemas.microsoft.com/office/word/2012/wordml":                   "w15",
	"http://schemas.microsoft.com/office/drawing/2010/main":                  "wp14",
	"http://schemas.openxmlformats.org/officeDocument/2006/math":             "m",
	"http://schemas.openxmlformats.org/package/2006/relationships":           "",
	"http://schemas.openxmlformats.org/package/2006/content-types":           "",
	"urn:schemas-microsoft-com:office:office":                                "o",
	"urn:schemas-microsoft-com:office:word":                                  "w",
	"urn:schemas-microsoft-com:office:spreadsheet":                           "x",
	"http://schemas.microsoft.com/office/word/2010/wordprocessingCanvas":     "wpc",
	"http://schemas.microsoft.com/office/word/2010/wordprocessingGroup":      "wpg",
	"http://schemas.microsoft.com/office/word/2010/wordprocessingInk":        "wpi",
	"http://schemas.microsoft.com/office/word/2010/wordprocessingShape":      "wps",
	"http://schemas.microsoft.com/office/word/2010/wordprocessingTextbox":    "wtx",
	"http://schemas.microsoft.com/office/word/2012/wordprocessingDrawing":    "wp15",
	"http://schemas.microsoft.com/office/drawing/2008/diagram":               "dgm",
	"http://schemas.openxmlformats.org/drawingml/2006/chart":                 "c",
	"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing":          "cd",
	"http://schemas.openxmlformats.org/drawingml/2006/lockedCanvas":          "lc",
	"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing":    "xdr",
	"http://schemas.microsoft.com/office/drawing/2010/picture":               "pic14",
}

func qualifiedName(name xml.Name) string {
	if name.Space == "" {
		return name.Local
	}
	if name.Space == "xml" {
		return "xml:" + name.Local
	}
	if name.Space == "xmlns" {
		return "xmlns:" + name.Local
	}
	prefix, ok := namespacePrefixes[name.Space]
	if !ok {
		return name.Local
	}
	if prefix == "" {
		return name.Local
	}
	return prefix + ":" + name.Local
}

func writeXMLNode(buffer *bytes.Buffer, node *xmlNode) {
	buffer.WriteString("<" + qualifiedName(node.Name))
	for _, attr := range node.Attr {
		buffer.WriteString(" " + qualifiedName(attr.Name) + "=\"")
		_ = xml.EscapeText(buffer, []byte(attr.Value))
		buffer.WriteString("\"")
	}
	buffer.WriteString(">")
	if node.Text != "" {
		_ = xml.EscapeText(buffer, []byte(node.Text))
	}
	for _, child := range node.Children {
		writeXMLNode(buffer, child)
	}
	buffer.WriteString("</" + qualifiedName(node.Name) + ">")
}

func (n *xmlNode) clone() *xmlNode {
	if n == nil {
		return nil
	}
	clone := &xmlNode{Name: n.Name, Attr: append([]xml.Attr(nil), n.Attr...), Text: n.Text}
	for _, child := range n.Children {
		clone.Children = append(clone.Children, child.clone())
	}
	return clone
}

func child(node *xmlNode, local string) *xmlNode {
	if node == nil {
		return nil
	}
	for _, child := range node.Children {
		if child.Name.Local == local {
			return child
		}
	}
	return nil
}

func children(node *xmlNode, local string) []*xmlNode {
	if node == nil {
		return nil
	}
	result := []*xmlNode{}
	for _, child := range node.Children {
		if child.Name.Local == local {
			result = append(result, child)
		}
	}
	return result
}

func descendants(node *xmlNode, local string) []*xmlNode {
	result := []*xmlNode{}
	if node == nil {
		return result
	}
	for _, child := range node.Children {
		if child.Name.Local == local {
			result = append(result, child)
		}
		result = append(result, descendants(child, local)...)
	}
	return result
}

func attr(node *xmlNode, local string) string {
	if node == nil {
		return ""
	}
	for _, attr := range node.Attr {
		if attr.Name.Local == local {
			return attr.Value
		}
	}
	return ""
}

func setAttr(node *xmlNode, namespace, local, value string) {
	for i := range node.Attr {
		if node.Attr[i].Name.Local == local && (namespace == "" || node.Attr[i].Name.Space == namespace) {
			node.Attr[i].Value = value
			return
		}
	}
	space := namespace
	if space == "" && local != "xmlns" && !strings.HasPrefix(local, "xmlns") {
		space = ""
	}
	node.Attr = append(node.Attr, xml.Attr{Name: xml.Name{Space: space, Local: local}, Value: value})
}
