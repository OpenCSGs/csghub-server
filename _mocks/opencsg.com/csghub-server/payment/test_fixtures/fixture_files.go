package test_fixtures

import (
	"embed"
	"path/filepath"
)

//go:embed files/*
var testFixtures embed.FS

func GetEmbeddedFile(fileName string) ([]byte, error) {
	return testFixtures.ReadFile(filepath.Join("files", fileName))
}
