package model

type SignInfo struct {
	Name            string
	Time            string
	Remark          string
	SignatureFileID string
}

type ReviewSignGroup struct {
	Name  string
	Signs []SignInfo
}

type SummaryRow struct {
	TypeName    string
	Total       int
	Normal      int
	InspectRate float64
	Problems    int
	Rectified   int
	RectifyRate float64
	Remark      string
}

type DetailRow struct {
	Location  string
	Marks     []string
	Problem   string
	Inspector string
	Time      string
	PhotoIDs  []string
}

type HeaderGroup struct {
	Label string
	Span  int
}

type HeaderField struct {
	Key   string
	Label string
	Value string
}

type DetailTable struct {
	TypeName     string
	TypeCode     string
	Items        []string
	ColumnKeys   []string
	HeaderGroups []HeaderGroup
	HeaderFields []HeaderField
	PhotoInline  bool
	Note         string
	Rows         []DetailRow
}

type LedgerRow struct {
	Date            string
	Category        string
	Location        string
	Problem         string
	ProblemPhotoIDs []string
	FixText         string
	Inspector       string
}

type PhotoCell struct {
	Label  string
	FileID string
}

type PhotoGroup struct {
	Title string
	Cells []PhotoCell
}

type ContactInfo struct {
	Tel     string
	Email   string
	Website string
	Address string
}

type MonthlyReportData struct {
	ReportNo         string
	CommunityName    string
	CommunityAddress string
	Period           string
	TitleLine        string
	CompanyName      string
	CompanyNameEn    string
	Contact          ContactInfo
	Approved         bool
	ApproveDate      string
	SealFileID       string
	TypeNames        []string
	Summary          []SummaryRow
	Details          []DetailTable
	PhotoGroups      []PhotoGroup
	Ledger           []LedgerRow
	IssueColumns     []string
	ReviewSigns      []ReviewSignGroup
	ImageLoader      func(fileID string) (data []byte, imgType string, err error)
}
