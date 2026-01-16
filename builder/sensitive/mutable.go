package sensitive

import (
	"log/slog"

	"opencsg.com/csghub-server/builder/sensitive/internal"
)

type Mutable struct {
	NewChecker func(data *internal.SensitiveWordData) SensitiveChecker
	SensitiveChecker
}

func NewMutableACAutomation(loader internal.Loader) SensitiveChecker {
	data, err := loader.Load()
	if err != nil {
		slog.Error("Failed to load sensitive data",
			slog.String("error", err.Error()))
	}
	checker := NewACAutomation(data)

	mutableChecker := &Mutable{
		NewChecker:       NewACAutomation,
		SensitiveChecker: checker,
	}
	loader.Subscribe(mutableChecker)
	return mutableChecker
}

func (m *Mutable) Update(data *internal.SensitiveWordData) error {
	m.SensitiveChecker = m.NewChecker(data)
	return nil
}
