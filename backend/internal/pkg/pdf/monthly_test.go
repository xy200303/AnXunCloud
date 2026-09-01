package pdf

import (
	"bytes"
	"fmt"
	"testing"
)

func TestLedgerMonthlyRendersDynamicTablesAndContinuationPages(t *testing.T) {
	rows := make([]DetailRow, 10)
	for index := range rows {
		rows[index] = DetailRow{Location: fmt.Sprintf("%d栋%d层灭火器", index/2+1, index+1), Marks: []string{"√", "√", "√", "√", "√"}, Inspector: "巡检员", Time: "2026-09-01 10:00"}
	}
	ledger := make([]LedgerRow, 7)
	for index := range ledger {
		ledger[index] = LedgerRow{Date: "2026-09-01", Problem: "压力表指针不在绿区", FixText: "待复核", Inspector: "巡检员"}
	}
	data := MonthlyReportData{
		CommunityName: "测试小区",
		Period:        "2026-09",
		Layout:        "ledger",
		TypeNames:     []string{"灭火器", "消防箱", "应急照明"},
		Summary:       []SummaryRow{{TypeName: "灭火器", Total: 10, Normal: 9, InspectRate: 100, Problems: 1}},
		Details:       []DetailTable{{TypeName: "灭火器", Items: []string{"压力", "瓶体", "喷管", "铅封", "有效期"}, Rows: rows}},
		Ledger:        ledger,
	}
	output, err := GenerateMonthly(data)
	if err != nil {
		t.Fatalf("GenerateMonthly() error = %v", err)
	}
	if !bytes.HasPrefix(output, []byte("%PDF")) {
		t.Fatal("generated output is not a PDF")
	}
	// 封面、目录、说明签字、汇总、明细两页（10 行每页 9 行）、台账两页（7 行每页 6 行），共八页。
	if pages := bytes.Count(output, []byte("/Type /Page\n")); pages != 8 {
		t.Fatalf("page count = %d, want 8", pages)
	}
}
