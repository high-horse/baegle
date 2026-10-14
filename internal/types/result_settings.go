package types

type ResultSettings struct {
	QueryTimeoutSeconds int   `json:"queryTimeoutSeconds"`
	RowsPerPage         int   `json:"rowsPerPage"`
	FontSize            int   `json:"fontSize"`
	SidebarFontSize     int   `json:"sidebarFontSize"`
	ConsoleFontSize     int   `json:"consoleFontSize"`
	ImportLimitMiB      int64 `json:"importLimitMiB"`
	StripedRows         bool  `json:"stripedRows"`
	ShowNulls           bool  `json:"showNulls"`
}

func DefaultResultSettings() ResultSettings {
	return ResultSettings{QueryTimeoutSeconds: 60, RowsPerPage: 100, FontSize: 11, SidebarFontSize: 12, ConsoleFontSize: 12, StripedRows: true}
}
