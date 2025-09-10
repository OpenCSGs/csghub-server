package payment

import (
	"fmt"
	"time"

	"github.com/bwmarrin/snowflake"
)

func GenerateOrderNumber() (string, error) {
	number, err := GenerateOrderNumberBySnowFlake(1)
	return number, err
}

func GenerateOrderNumberBySnowFlake(nodeNum int64) (string, error) {
	node, err := snowflake.NewNode(nodeNum)
	if err != nil {
		return "", err
	}
	id := node.Generate()

	date := time.Now().Format("20060102")

	orderNumber := fmt.Sprintf("%s%d", date, id)

	return orderNumber, nil
}
