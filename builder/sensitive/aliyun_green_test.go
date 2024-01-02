package sensitive

import (
	"fmt"
	"math"
	"os"
	"testing"
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
