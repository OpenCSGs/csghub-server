package types

type CONSUMER_INFO struct {
	ConsumerID    string `json:"customer_id"`
	ConsumerPrice string `json:"customer_price"`
	PriceUnit     string `json:"price_unit"`
	Duration      string `json:"customer_duration"`
}

var (
	SceneReserve        int = 0  // system reserve
	ScenePortalCharge   int = 1  // portal charge fee
	SceneModelInference int = 10 // model inference endpoint
	SceneSpace          int = 11 // csghub space
	SceneModelFinetune  int = 12 // model finetune
	SceneStarship       int = 20 // starship
	SceneUnknow         int = 99 // unknow
)
