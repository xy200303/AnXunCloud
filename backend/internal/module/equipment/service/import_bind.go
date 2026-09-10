package service

import (
	"regexp"
	"strings"

	insmodel "anxuncloud/internal/module/inspection/model"
)

// ========== 导入时点位自动推理绑定（戎安台账实测驱动；两侧共用同一归一化） ==========
//
// 算法：设备文本（安装位置 → 管控区域 → 设备名称 → 机房名称，取第一个同时解析出楼栋+楼层的）
// 与点位（楼栋名称归一化/无楼栋时从点位名解析 + 点位名解析楼层）归一成同一键 "b{楼栋}|f{楼层}"，
// 恰好 1 个候选点位才绑（0=无对应，>1=歧义，都跳过）；只填 NULL 的 point_id，绝不动人工已绑。

// buildingRe 楼栋号：数字 + 栋/号楼/#楼/#（"11栋" 整体捕获为 11，不被 "1栋" 误匹配——\d+ 贪婪）。
var buildingRe = regexp.MustCompile(`(\d+)\s*(?:栋|号楼|#楼|#)`)

// floorNegRe 负楼层：负/B + 数字 + 楼/层（负1层/B2 楼）。
var floorNegRe = regexp.MustCompile(`(?:负|B)(\d+)\s*(?:楼|层)`)

// floorRe 正楼层：数字 + 楼/层/F（F 后紧跟英文字母时跳过该匹配，防误吃型号；RE2 不支持 lookahead，用索引手工判定）。
var floorRe = regexp.MustCompile(`(\d+)\s*(楼|层|F)`)

// normalizeLocText 归一化：全角数字/字母转半角、去空白。
func normalizeLocText(s string) string {
	out := strings.Map(func(r rune) rune {
		switch {
		case r >= '０' && r <= '９': // 全角数字
			return r - '０' + '0'
		case r >= 'Ａ' && r <= 'Ｚ': // 全角大写
			return r - 'Ａ' + 'A'
		case r >= 'ａ' && r <= 'ｚ': // 全角小写
			return r - 'ａ' + 'a'
		case r == ' ' || r == '\t' || r == '　': // 半角/全角空白
			return -1
		}
		return r
	}, s)
	return out
}

// parseBuildingKey 楼栋键："b{n}"（数字楼栋）或 "b车库"（含车库关键词；仅当小区存在同名楼栋时才可能命中）。
func parseBuildingKey(text string) string {
	text = normalizeLocText(text)
	if m := buildingRe.FindStringSubmatch(text); m != nil {
		n := strings.TrimLeft(m[1], "0") // 去前导零（01栋=1栋）
		if n == "" {
			n = "0"
		}
		return "b" + n
	}
	if strings.Contains(text, "车库") {
		return "b车库"
	}
	return ""
}

// parseFloor 楼层：架空层→0；负n/Bn→-n；n楼/n层/nF→n（nF 后紧跟英文字母视为型号，跳过）。无法解析返回 (0, false)。
func parseFloor(text string) (int, bool) {
	text = normalizeLocText(text)
	if strings.Contains(text, "架空层") {
		return 0, true
	}
	if m := floorNegRe.FindStringSubmatch(text); m != nil {
		n := atoiSafe(m[1])
		return -n, true
	}
	for _, m := range floorRe.FindAllStringSubmatchIndex(text, -1) {
		// m: [匹配起, 匹配止, 数字组起, 数字组止, 后缀组起, 后缀组止]
		suffix := text[m[4]:m[5]]
		if suffix == "F" && m[1] < len(text) {
			nxt := text[m[1]]
			if (nxt >= 'a' && nxt <= 'z') || (nxt >= 'A' && nxt <= 'Z') {
				continue // 型号里的 F（如 4F-xxx 后的字母段不算楼层）
			}
		}
		return atoiSafe(text[m[2]:m[3]]), true
	}
	return 0, false
}

func atoiSafe(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// locKey 位置键：楼栋|楼层。
func locKey(building string, floor int) string {
	return building + "|f" + itoa(floor)
}

func itoa(n int) string {
	if n < 0 {
		return "-" + itoaAbs(-n)
	}
	return itoaAbs(n)
}

func itoaAbs(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// parseDeviceLocation 设备侧：按优先级（安装位置 → 管控区域 → 设备名称 → 机房名称）
// 取第一个同时解析出楼栋+楼层的文本。多段数字楼栋范围（"1-5栋楼道"）楼栋键只取到 5栋 且无楼层 → 保守跳过。
func parseDeviceLocation(texts []string) (string, bool) {
	for _, t := range texts {
		if t == "" {
			continue
		}
		b := parseBuildingKey(t)
		if b == "" {
			continue
		}
		f, ok := parseFloor(t)
		if !ok {
			continue
		}
		return locKey(b, f), true
	}
	return "", false
}

// pointLocationKey 点位侧键：楼栋取所属楼栋名称归一化（无楼栋信息时从点位名解析），楼层从点位名解析。
func pointLocationKey(pointName, buildingName string) (string, bool) {
	b := ""
	if buildingName != "" {
		b = parseBuildingKey(buildingName)
	}
	if b == "" {
		b = parseBuildingKey(pointName)
	}
	if b == "" {
		return "", false
	}
	f, ok := parseFloor(pointName)
	if !ok {
		return "", false
	}
	return locKey(b, f), true
}

// pointBindIndex 点位绑定索引：键 → 候选点位（恰好 1 个才可绑）。
type pointBindIndex struct {
	byKey    map[string][]bindCandidate
	building map[string]*string // 楼栋键 → 楼栋 id（绑定点位时顺带补 building_id）
}

type bindCandidate struct {
	pointID    string
	buildingID *string
}

// buildPointBindIndex 建索引：楼栋全量 + 点位全量（按小区）。
func buildPointBindIndex(buildings []insmodel.Building, points []insmodel.InspectionPoint) *pointBindIndex {
	idx := &pointBindIndex{byKey: map[string][]bindCandidate{}, building: map[string]*string{}}
	buildingName := map[string]string{}
	for i := range buildings {
		b := &buildings[i]
		buildingName[b.ID] = b.Name
		bk := parseBuildingKey(b.Name)
		if bk != "" {
			id := b.ID
			idx.building[bk] = &id
		}
	}
	for i := range points {
		p := &points[i]
		bName := ""
		if p.BuildingID != nil {
			bName = buildingName[*p.BuildingID]
		}
		key, ok := pointLocationKey(p.Name, bName)
		if !ok {
			continue
		}
		idx.byKey[key] = append(idx.byKey[key], bindCandidate{pointID: p.ID, buildingID: p.BuildingID})
	}
	return idx
}

// lookup 恰好 1 个候选才返回（0=无对应，>1=歧义）。
func (idx *pointBindIndex) lookup(key string) *bindCandidate {
	cands := idx.byKey[key]
	if len(cands) != 1 {
		return nil
	}
	return &cands[0]
}
