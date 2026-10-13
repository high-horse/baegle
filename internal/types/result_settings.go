package types

type ResultSettings struct {
	RowsPerPage     int  `json:"rowsPerPage"`
	FontSize        int  `json:"fontSize"`
	SidebarFontSize int  `json:"sidebarFontSize"`
	ConsoleFontSize int  `json:"consoleFontSize"`
	StripedRows     bool `json:"stripedRows"`
	ShowNulls       bool `json:"showNulls"`
}

func DefaultResultSettings() ResultSettings {
	return ResultSettings{RowsPerPage: 100, FontSize: 11, SidebarFontSize: 12, ConsoleFontSize: 12, StripedRows: true}
}
