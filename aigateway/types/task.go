package types

import (
	commontypes "opencsg.com/csghub-server/common/types"
)

// IsAutoSpeechRecognition reports whether the pipeline tasks describe an
// automatic speech recognition model. It accepts both the canonical Hugging Face
// task name and the legacy alias.
func IsAutoSpeechRecognition(tasks []string) bool {
	for _, task := range tasks {
		switch task {
		case string(commontypes.AutomaticSpeechRecognition), string(commontypes.AutoSpeechRecognition):
			return true
		}
	}
	return false
}
