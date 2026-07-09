package audio

import (
	"net/http"
	"strconv"
	"strings"

	"opencsg.com/csghub-server/aigateway/types"
)

const audioDurationHeader = "Audio-Duration-Seconds"

type FunASRAdapter struct{}

func NewFunASRAdapter() *FunASRAdapter {
	return &FunASRAdapter{}
}

func (a *FunASRAdapter) Name() string {
	return "funasr"
}

func (a *FunASRAdapter) CanHandle(model *types.Model) bool {
	return model != nil && (isValue(model.RuntimeFramework, "funasr") || isValue(model.Provider, "opencsg"))
}

func (a *FunASRAdapter) DurationFromHeader(header http.Header) (float64, bool) {
	return parseDurationHeader(header)
}

func parseDurationHeader(header http.Header) (float64, bool) {
	if header == nil {
		return 0, false
	}
	value := header.Get(audioDurationHeader)
	if value == "" {
		return 0, false
	}
	duration, err := strconv.ParseFloat(value, 64)
	if err != nil || duration <= 0 {
		return 0, false
	}
	return duration, true
}

func isValue(value, expected string) bool {
	return strings.EqualFold(strings.TrimSpace(value), expected)
}
