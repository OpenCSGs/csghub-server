package checker

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	mockio "opencsg.com/csghub-server/_mocks/io"
	mocksens "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/builder/sensitive"
	"opencsg.com/csghub-server/common/types"
)

func TestTextFileChecker_Run(t *testing.T) {

	t.Run("contains sensitive words", func(t *testing.T) {
		localWordChecker = NewDFA()
		localWordChecker.BuildDFA(getSensitiveWordList(`5pWP5oSf6K+NLHNlbnNpdGl2ZXdvcmQ=`))
		checker := NewTextFileChecker()

		reader1 := bytes.NewReader([]byte("This text contains sensitive word."))
		expectedStatus1 := types.SensitiveCheckFail
		expectedMessage1 := "contains sensitive word"
		status1, message1 := checker.Run(reader1)
		if status1 != expectedStatus1 || message1 != expectedMessage1 {
			t.Errorf("Test case 1 failed: Expected (%v, %v), Got (%v, %v)", expectedStatus1, expectedMessage1, status1, message1)
		}
	})
	t.Run("no sensitive words", func(t *testing.T) {
		localWordChecker = NewDFA()
		localWordChecker.BuildDFA(getSensitiveWordList(`5pWP5oSf6K+NLHNlbnNpdGl2ZXdvcmQ=`))
		mockContentChecker := mocksens.NewMockSensitiveChecker(t)
		contentChecker = mockContentChecker
		mockContentChecker.EXPECT().PassTextCheck(mock.Anything, mock.Anything, mock.Anything).Return(&sensitive.CheckResult{
			IsSensitive: false,
			Reason:      "",
		}, nil)
		checker := NewTextFileChecker()

		reader2 := bytes.NewReader([]byte("This is a regular text file."))
		expectedStatus2 := types.SensitiveCheckPass
		expectedMessage2 := ""
		status2, message2 := checker.Run(reader2)
		if status2 != expectedStatus2 || message2 != expectedMessage2 {
			t.Errorf("Test case 2 failed: Expected (%v, %v), Got (%v, %v)", expectedStatus2, expectedMessage2, status2, message2)
		}
	})

	t.Run("failed to read file content", func(t *testing.T) {
		checker := NewTextFileChecker()

		reader := mockio.NewMockReader(t)
		reader.EXPECT().Read(mock.Anything).Return(0, errors.New("failed to read file content"))
		expectedStatus3 := types.SensitiveCheckException
		expectedMessage3 := "failed to read file content"
		status3, message3 := checker.Run(reader)
		if status3 != expectedStatus3 || message3 != expectedMessage3 {
			t.Errorf("Test case 3 failed: Expected (%v, %v), Got (%v, %v)", expectedStatus3, expectedMessage3, status3, message3)
		}
	})
}
