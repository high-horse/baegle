package types

type SavedConsole struct {
	ConnectionID int    `json:"connectionId"`
	ID           string `json:"id"`
	Title        string `json:"title"`
	SQL          string `json:"sql"`
	Position     int    `json:"position"`
	CreatedAt    int64  `json:"createdAt"`
	TabState     string `json:"tabState"`
}
