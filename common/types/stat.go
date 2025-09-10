package types

type StatTargetType string // table name

const (
	StatTargetUser         StatTargetType = "users"
	StatTargetOrganization StatTargetType = "organizations"

	StatTargetModel   StatTargetType = "models"
	StatTargetDataset StatTargetType = "datasets"
	StatTargetMcp     StatTargetType = "mcp_servers"
	StatTargetSpace   StatTargetType = "spaces"
	StatTargetCode    StatTargetType = "codes"
	StatTargetPrompt  StatTargetType = "prompts"
)

var AllStatTargetTypes = []StatTargetType{
	StatTargetUser, StatTargetOrganization,
	StatTargetModel, StatTargetDataset, StatTargetMcp, StatTargetSpace, StatTargetCode, StatTargetPrompt,
}

func IsValidStatTargetType(t string) bool {
	for _, valid := range AllStatTargetTypes {
		if string(valid) == t {
			return true
		}
	}
	return false
}

type StatDateType string

const (
	StatDateYear  StatDateType = "year"
	StatDateMonth StatDateType = "month"
	StatDateWeek  StatDateType = "week"
	StatDateDay   StatDateType = "day"
)

var validDateTypes = map[StatDateType]struct{}{
	StatDateYear:  {},
	StatDateMonth: {},
	StatDateWeek:  {},
	StatDateDay:   {},
}

func IsValidStatDateType(d string) bool {
	_, ok := validDateTypes[StatDateType(d)]
	return ok
}

type StatSnapshotReq struct {
	TargetType   StatTargetType `json:"target_type"`
	DateType     StatDateType   `json:"date_type"`
	SnapshotDate string         `json:"snapshot_date"`
}

type StatSnapshotResp struct {
	ID           int64          `json:"id"`
	TargetType   string         `json:"target_type"`
	DateType     string         `json:"date_type"`
	SnapshotDate string         `json:"snapshot_date"`
	TrendData    map[string]int `json:"trend_data"`
	TotalCount   int            `json:"total_count"`
	NewCount     int            `json:"new_count"`
}
