package sensitive

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"

	"opencsg.com/csghub-server/common/config"
)

func Test_splitTasks(t *testing.T) {
	const txtFileName = "./large_text_to_check.txt"
	c := new(AliyunGreenChecker)
	buf, err := os.ReadFile(txtFileName)
	if err != nil {
		t.Log("Failed to read large text:", txtFileName)
		t.FailNow()
	}
	largeText := string(buf)
	tasks := c.splitTasks(largeText)
	taskCount := math.Round(float64(len(largeText)) / float64(1000))
	fmt.Println(taskCount, len(tasks))
	if len(tasks) != int(taskCount) {
		t.Logf("task count mismatch, expected: %d, got: %d", int(taskCount), len(tasks))
		t.FailNow()
	}
}

func Test_passLargeTextCheck(t *testing.T) {
	const txtFileName = "./large_text_to_check.txt"
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Log("Failed to load config:", err)
		t.FailNow()
	}
	cfg.SensitiveCheck.Enable = true
	c := NewAliyunGreenChecker(cfg)
	buf, err := os.ReadFile(txtFileName)
	if err != nil {
		t.Log("Failed to read large text:", txtFileName)
		t.FailNow()
	}
	largeText := string(buf)
	tasks := c.splitTasks(largeText)
	content, _ := json.Marshal(
		map[string]interface{}{
			"scenes": [...]string{"antispam"},
			"tasks":  tasks,
		},
	)

	success, err := c.passLargeTextCheck(context.Background(), string(content))

	if err != nil {
		t.Log(err.Error())
		t.FailNow()
	}

	if !success {
		t.Log("success", success)
		t.FailNow()
	}
}

func Test_PassTextCheck(t *testing.T) {
	cfg, err := config.LoadConfig()
	if err != nil {
		t.Log("Failed to load config:", err)
		t.FailNow()
	}
	cfg.SensitiveCheck.Enable = true
	c := NewAliyunGreenChecker(cfg)
	content := "http://github.com/repo"
	pass, err := c.PassTextCheck(context.Background(), ScenarioCommentDetection, content)
	if err != nil {
		t.Fail()
	}
	if !pass {
		t.Log("fail")
		t.FailNow()
	}
}
