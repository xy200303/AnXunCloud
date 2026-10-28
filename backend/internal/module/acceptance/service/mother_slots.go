package service

type acceptanceTableSpec struct {
	table    int
	photo    int
	kind     string
	embedded bool
	twoUp    bool
}

func listSlot(table, photo int) acceptanceTableSpec {
	return acceptanceTableSpec{table: table, photo: photo, kind: "list", embedded: table == photo}
}

func documentSlot(table, photo int) acceptanceTableSpec {
	return acceptanceTableSpec{table: table, photo: photo, kind: "list", twoUp: true}
}

func matrixSlot(table, photo int) acceptanceTableSpec {
	return acceptanceTableSpec{table: table, photo: photo, kind: "matrix", embedded: table == photo}
}

var acceptanceTableSlots = map[string]acceptanceTableSpec{
	"documents_project":     documentSlot(4, 5),
	"documents_engineering": documentSlot(6, 7),
	"documents_owner":       documentSlot(8, 9),
	"documents_operation":   documentSlot(10, 11),
	"documents_title":       documentSlot(12, 13),
	"documents_keys":        documentSlot(14, 15),

	"building_1_exterior": matrixSlot(4, 5),
	"building_1_common":   matrixSlot(6, 7),
	"building_2_exterior": matrixSlot(8, 9),
	"building_2_common":   matrixSlot(10, 11),
	"building_3_exterior": matrixSlot(12, 13),
	"building_3_common":   matrixSlot(14, 15),
	"building_4_exterior": matrixSlot(16, 17),
	"building_4_common":   matrixSlot(18, 19),

	"facilities_fire_1":       matrixSlot(4, 5),
	"facilities_fire_2":       matrixSlot(6, 7),
	"facilities_fire_3":       matrixSlot(8, 9),
	"facilities_fire_4":       matrixSlot(10, 11),
	"facilities_fire_control": matrixSlot(12, 12),
	"facilities_fire_system":  matrixSlot(13, 13),
	"facilities_hydrant":      matrixSlot(14, 15),
	"facilities_sprinkler":    matrixSlot(16, 17),
	"facilities_broadcast":    matrixSlot(18, 19),
	"facilities_phone":        matrixSlot(20, 21),
	"facilities_fan":          matrixSlot(22, 23),
	"facilities_security":     matrixSlot(24, 24),
	"facilities_camera":       matrixSlot(25, 25),
	"facilities_intercom":     matrixSlot(26, 27),
	"facilities_perimeter":    matrixSlot(28, 29),
	"facilities_gate":         matrixSlot(30, 30),
	"facilities_water":        matrixSlot(31, 31),
	"facilities_power":        matrixSlot(32, 32),
	"facilities_elevator":     matrixSlot(33, 34),
	"facilities_mechanical":   matrixSlot(35, 35),

	"outdoor_garden":     listSlot(4, 5),
	"outdoor_hardscape":  listSlot(6, 7),
	"outdoor_recreation": listSlot(8, 9),
	"outdoor_drainage":   listSlot(10, 11),
	"outdoor_sanitation": listSlot(12, 13),
	"outdoor_signage":    listSlot(14, 15),
	"outdoor_lighting":   listSlot(16, 17),
	"outdoor_irrigation": listSlot(18, 19),

	"premises_documents": listSlot(4, 5),
	"premises_building":  listSlot(6, 7),
	"premises_assets":    listSlot(8, 9),

	"garage_documents":     listSlot(4, 4),
	"garage_structure":     matrixSlot(5, 5),
	"garage_drainage":      matrixSlot(6, 6),
	"garage_traffic":       matrixSlot(7, 7),
	"garage_lighting":      matrixSlot(8, 8),
	"garage_fire":          matrixSlot(9, 9),
	"garage_ventilation":   matrixSlot(10, 10),
	"garage_camera":        matrixSlot(11, 12),
	"garage_access":        matrixSlot(13, 13),
	"garage_civil_defense": matrixSlot(14, 14),
}
