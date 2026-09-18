package pdf

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// mockPNG 生成一张纯色 PNG（样例测试的签名/公章/现场照片占位图）。
func mockPNG(c color.RGBA) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 240, 160))
	for y := 0; y < 160; y++ {
		for x := 0; x < 240; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// TestMonthlySamplePDF 渲染一份带模拟数据的月报样例，留存 backend/tmp/sample_monthly.pdf 供版面核对。
// 运行：cd backend && go test ./internal/pkg/pdf/ -run TestMonthlySamplePDF
func TestMonthlySamplePDF(t *testing.T) {
	photo := mockPNG(color.RGBA{120, 140, 160, 255})
	seal := mockPNG(color.RGBA{200, 40, 40, 255})
	sign := mockPNG(color.RGBA{250, 250, 250, 255})
	loader := func(fileID string) ([]byte, string, error) {
		switch fileID {
		case "seal":
			return seal, "PNG", nil
		case "sign":
			return sign, "PNG", nil
		}
		return photo, "PNG", nil
	}

	extinguisherRows := make([]DetailRow, 11)
	for i := range extinguisherRows {
		marks := []string{"√", "√", "√", "√", "√"}
		if i == 3 {
			marks[0] = "×"
		}
		extinguisherRows[i] = DetailRow{
			Location:  fmt.Sprintf("%d栋%d层灭火器箱 MH-%03d", i/3+1, i+1, i+1),
			Marks:     marks,
			Inspector: "张伟",
			Time:      fmt.Sprintf("09-%02d 1%d:05", i+1, i%10),
		}
	}
	hydrantRows := make([]DetailRow, 8)
	for i := range hydrantRows {
		hydrantRows[i] = DetailRow{
			Location:  fmt.Sprintf("%d栋大堂消火栓 XHS-%03d", i/4+1, i+1),
			Marks:     []string{"√", "√", "√", "√", "√", "√"},
			Inspector: "李娜",
			Time:      fmt.Sprintf("09-%02d 09:30", i+2),
		}
	}
	lightRows := make([]DetailRow, 6)
	for i := range lightRows {
		lightRows[i] = DetailRow{
			Location:  fmt.Sprintf("%d栋疏散通道", i+1),
			Marks:     []string{"√", "√", "√"},
			Inspector: "王强",
			Time:      fmt.Sprintf("09-%02d 14:20", i+3),
		}
	}

	data := MonthlyReportData{
		ReportNo:         "雄楚春天-2026-09",
		CommunityName:    "雄楚春天",
		CommunityAddress: "武汉市洪山区雄楚大道 100 号",
		Period:           "2026-09",
		TitleLine:        "物业消防设施（器材类）月度",
		CompanyName:      "武汉一品行物业有限公司开发区分公司",
		CompanyNameEn:    "Wuhan Yipinxing Property Co., Ltd. Development Zone Branch",
		Contact: ContactInfo{
			Tel:     "027-88668888",
			Email:   "ephkf@163.com",
			Website: "http://www.ephwy.com/",
			Address: "武汉市洪山区野芷湖西路创意天地07创意工坊202",
		},
		Approved:   true,
		SealFileID: "seal",
		TypeNames:  []string{"消火栓", "灭火器", "应急照明灯"},
		Summary: []SummaryRow{
			{TypeName: "消火栓（箱）", Total: 8, Normal: 8, InspectRate: 100, Problems: 0, Rectified: 0, RectifyRate: 0},
			{TypeName: "灭火器（箱）", Total: 11, Normal: 10, InspectRate: 100, Problems: 1, Rectified: 1, RectifyRate: 100},
			{TypeName: "应急照明灯", Total: 6, Normal: 6, InspectRate: 100, Problems: 0, Rectified: 0, RectifyRate: 0},
		},
		Details: []DetailTable{
			{TypeName: "灭火器", Items: []string{"压力", "瓶体", "喷管", "铅封", "有效期"}, Rows: extinguisherRows},
			{TypeName: "消火栓", Items: []string{"箱门", "水带", "枪头", "接口", "水压", "周围"}, Rows: hydrantRows},
			{TypeName: "应急照明灯", Items: []string{"灯具完好", "指示清晰", "通道畅通"}, Rows: lightRows},
		},
		Ledger: []LedgerRow{
			{Category: "灭火器", Location: "2栋4层灭火器箱 MH-004", Problem: "压力表指针不在绿区", ProblemPhotoIDs: []string{"p1"}, FixText: "复核通过"},
			{Category: "消火栓", Location: "1栋大堂消火栓 XHS-002", Problem: "箱门玻璃破损", ProblemPhotoIDs: []string{"p2"}, FixText: "复核通过"},
			{Category: "应急照明灯", Location: "3栋疏散通道", Problem: "灯具不亮", ProblemPhotoIDs: []string{"p3"}, FixText: "待复核"},
		},
		ReviewSigns: []ReviewSignGroup{
			{Name: "巡检", Signs: []SignInfo{{Name: "张伟", Time: "2026-09-30 10:00:00", SignatureFileID: "sign"}}},
			{Name: "审核", Signs: []SignInfo{{Name: "李娜", Time: "2026-09-30 15:00:00"}}},
			{Name: "签批", Signs: []SignInfo{{Name: "", Time: ""}}},
		},
		PhotoGroups: []PhotoGroup{
			{Title: "灭火器", Cells: []PhotoCell{{Label: "1栋1层灭火器·09-01·压力", FileID: "s1"}, {Label: "1栋2层灭火器·09-02·瓶体", FileID: "s2"}, {Label: "2栋4层灭火器·09-04·压力", FileID: "s3"}, {Label: "3栋1层灭火器·09-07·铅封", FileID: "s4"}}},
		},
		ImageLoader: loader,
	}
	output, err := GenerateMonthly(data)
	if err != nil {
		t.Fatalf("GenerateMonthly() error = %v", err)
	}
	out := filepath.Join("..", "..", "..", "tmp", "sample_monthly.pdf")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		t.Fatalf("mkdir tmp: %v", err)
	}
	if err := os.WriteFile(out, output, 0o644); err != nil {
		t.Fatalf("write sample pdf: %v", err)
	}
	t.Logf("样例月报已生成: %s（%d 字节）", out, len(output))
}
