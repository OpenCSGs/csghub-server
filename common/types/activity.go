package types

type ACTIVITY_REQ struct {
	ID     int64   `json:"id" binding:"min=1"`        // activate ID
	Value  float64 `json:"value" binding:"min=1"`     // charge credit number for activity
	OpUID  string  `json:"op_uid" binding:"required"` // operator id
	OpDesc string  `json:"desc" binding:"required"`   // activate description
}

var StarShipNewUser = ACTIVITY_REQ{
	ID:     1001,
	Value:  10000, // 100 credit
	OpUID:  "",    // fill in user name
	OpDesc: "create starship access token for first time",
}
