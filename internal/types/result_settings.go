package types

type ResultSettings struct {
	RowsPerPage int  `json:"rowsPerPage"`
	FontSize    int  `json:"fontSize"`
	StripedRows bool `json:"stripedRows"`
	ShowNulls   bool `json:"showNulls"`
}

func DefaultResultSettings() ResultSettings {
	return ResultSettings{RowsPerPage: 100, FontSize: 11, StripedRows: true}
}
