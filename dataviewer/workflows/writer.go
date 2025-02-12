package workflows

import (
	"os"
)

type FileWriterImpl struct {
	file       *os.File
	writeBytes int
}

func NewFileWriter(file *os.File) *FileWriterImpl {
	return &FileWriterImpl{
		file: file,
	}
}

func (fw *FileWriterImpl) Write(p []byte) (n int, err error) {
	fw.writeBytes, err = fw.file.Write(p)
	return fw.writeBytes, err
}

func (fw *FileWriterImpl) GetWriteBytes() int {
	return fw.writeBytes
}
