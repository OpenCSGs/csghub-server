package rsa

import (
	"fmt"
	"os"
)

type KeysReader interface {
	ReadKey(fileName string) ([]byte, error)
}

type keysReaderImpl struct {
}

func NewKeysReader() KeysReader {
	return &keysReaderImpl{}
}

func (k *keysReaderImpl) ReadKey(fileName string) ([]byte, error) {
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		return nil, fmt.Errorf("file %s does not exist", fileName)
	}

	contents, err := os.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("read file %s with error: %w", fileName, err)
	}
	return contents, nil
}
