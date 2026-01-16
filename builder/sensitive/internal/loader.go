package internal

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"opencsg.com/csghub-server/common/config"
)

// SensitiveWordData represents the sensitive word data
type SensitiveWordData struct {
	TagMap map[int]string
	Words  []string
}

// Loader interface for loading sensitive words from different sources
type Loader interface {
	Load() (*SensitiveWordData, error)
	Subscribe(observer Observer)
	Unsubscribe(observer Observer)
	NotifyObservers()
}

// Observer interface for sensitive word updates
// Used by loaders to notify subscribers of data changes
type Observer interface {
	Update(data *SensitiveWordData) error
}

// Base provides common functionality for all loaders and can act as a static data loader
type Base struct {
	mu        sync.RWMutex
	observers []Observer
	data      *SensitiveWordData
}

// Subscribe adds an observer to the loader
func (bl *Base) Subscribe(observer Observer) {
	bl.mu.Lock()
	defer bl.mu.Unlock()
	bl.observers = append(bl.observers, observer)
	slog.Debug("observer subscribed to loader", slog.Int("observer_count", len(bl.observers)))
}

// Unsubscribe removes an observer from the loader
func (bl *Base) Unsubscribe(observer Observer) {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	for i, obs := range bl.observers {
		if obs == observer {
			bl.observers = append(bl.observers[:i], bl.observers[i+1:]...)
			break
		}
	}
	slog.Debug("observer unsubscribed from loader", slog.Int("observer_count", len(bl.observers)))
}

// Load loads sensitive words from static data
func (bl *Base) Load() (*SensitiveWordData, error) {
	bl.mu.RLock()
	defer bl.mu.RUnlock()
	data := bl.data
	return data, nil
}

// NewBaseLoader creates a new static data loader
func NewBaseLoader(tagMap map[int]string, words []string) Loader {
	return &Base{
		data: &SensitiveWordData{
			TagMap: tagMap,
			Words:  words,
		},
	}
}

// NotifyObservers notifies all observers with new data
func (bl *Base) NotifyObservers() {
	bl.mu.RLock()
	observers := make([]Observer, len(bl.observers))
	copy(observers, bl.observers)
	bl.mu.RUnlock()

	for _, observer := range observers {
		if err := observer.Update(bl.data); err != nil {
			slog.Error("failed to notify observer", slog.String("error", err.Error()))
		}
	}
}

// ConfigLoader loads sensitive words from configuration files
type ConfigLoader struct {
	Base
	configPath string
}

func (cl *ConfigLoader) Name() string {
	return "config_loader"
}

// NewConfigLoader creates a new configuration-based loader
func NewConfigLoader(config *config.Config) Loader {
	return &ConfigLoader{
		configPath: config.SensitiveCheck.DictDir,
	}
}

// Load loads sensitive words from configuration file
func (cl *ConfigLoader) Load() (*SensitiveWordData, error) {
	slog.Info("loading sensitive words from config", slog.String("path", cl.configPath))

	data := &SensitiveWordData{
		TagMap: make(map[int]string),
		Words:  []string{},
	}

	currentIndex := 0
	err := filepath.WalkDir(cl.configPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".txt") {
			parent := filepath.Base(filepath.Dir(path))
			tag := strings.TrimSuffix(d.Name(), ".txt")
			if parent != "." && parent != "" && parent != filepath.Base(cl.configPath) {
				tag = parent
			}

			content, readErr := os.ReadFile(path)
			if readErr != nil {
				slog.Error("failed to read dict file", slog.String("path", path), slog.Any("error", readErr))
				return nil
			}
			lines := strings.Split(string(content), "\n")
			for _, w := range lines {
				w = strings.TrimSpace(w)
				if w == "" {
					continue
				}
				data.Words = append(data.Words, w)
				data.TagMap[currentIndex] = tag
				currentIndex++
			}
		}
		return nil
	})
	if err != nil {
		slog.Error("failed to walk dict dir", slog.String("dir", cl.configPath), slog.Any("error", err))
	}

	slog.Info("sensitive words loaded from config", slog.Int("word_count", len(data.Words)))
	cl.data = data
	// Notify all observers about the new data
	cl.NotifyObservers()

	return data, nil
}
