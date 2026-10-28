package ooxml

import "embed"

//go:embed templates/*.docx
var templateFiles embed.FS

var builtinTemplateIDs = map[string]string{
	"fire_equipment":        "93000000-0000-0000-0000-000000000001",
	"fire_system":           "93000000-0000-0000-0000-000000000002",
	"supply":                "93000000-0000-0000-0000-000000000003",
	"power":                 "93000000-0000-0000-0000-000000000004",
	"special":               "93000000-0000-0000-0000-000000000005",
	"acceptance_documents":  "94000000-0000-0000-0000-000000000001",
	"acceptance_building":   "94000000-0000-0000-0000-000000000002",
	"acceptance_outdoor":    "94000000-0000-0000-0000-000000000003",
	"acceptance_facilities": "94000000-0000-0000-0000-000000000004",
	"acceptance_garage":     "94000000-0000-0000-0000-000000000005",
	"acceptance_premises":   "94000000-0000-0000-0000-000000000006",
	"acceptance_unit":       "94000000-0000-0000-0000-000000000007",
}

func BuiltinTemplate(volumeCode string) ([]byte, string, bool) {
	fileID, ok := builtinTemplateIDs[volumeCode]
	if !ok {
		return nil, "", false
	}
	data, err := templateFiles.ReadFile("templates/" + volumeCode + ".docx")
	return data, fileID, err == nil
}
