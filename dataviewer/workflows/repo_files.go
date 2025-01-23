package workflows

import (
	"context"
	"fmt"
	"strings"

	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/common/types"
	dvCom "opencsg.com/csghub-server/dataviewer/common"
)

const (
	RepoParquetData dvCom.RepoDataType = "parquet"
	RepoJsonData    dvCom.RepoDataType = "json"
	RepoCsvData     dvCom.RepoDataType = "csv"
)

var (
	GitDefaultUserName  = "admin"
	GitDefaultUserEmail = "admin@csghub.com"
	DefaultSubsetName   = "default"
	TrainSplitName      = "train"
	TestSplitName       = "test"
	ValSplitName        = "validation"
	OtherSplitName      = "others"
)

func IsValidParquetFile(entryName string) bool {
	return strings.HasSuffix(strings.ToLower(entryName), ".parquet")
}

func IsValidJsonFile(entryName string) bool {
	return strings.HasSuffix(strings.ToLower(entryName), ".jsonl") || strings.HasSuffix(strings.ToLower(entryName), ".json")
}

func IsValidCSVFile(entryName string) bool {
	return strings.HasSuffix(strings.ToLower(entryName), ".csv")
}

func IsTrainFile(fileName string) bool {
	fileName = strings.ToLower(fileName)
	if strings.Contains(fileName, "train") || strings.Contains(fileName, "training") {
		return true
	}
	return false
}

func IsTestFile(fileName string) bool {
	fileName = strings.ToLower(fileName)
	if strings.Contains(fileName, "test") || strings.Contains(fileName, "testing") {
		return true
	}
	if strings.Contains(fileName, "eval") || strings.Contains(fileName, "evaluation") {
		return true
	}
	return false
}

func IsValidationFile(fileName string) bool {
	fileName = strings.ToLower(fileName)
	if strings.Contains(fileName, "val") || strings.Contains(fileName, "valid") || strings.Contains(fileName, "validation") {
		return true
	}
	if strings.Contains(fileName, "dev") {
		return true
	}
	return false
}

func GetFilePaths(req dvCom.RepoFilesReq, fileClass *dvCom.RepoFilesClass, maxFileSize int64) error {
	err := getAllFiles(req, fileClass, maxFileSize)
	if err != nil {
		return fmt.Errorf("get repo all files error: %w", err)
	}
	return nil
}

func getAllFiles(req dvCom.RepoFilesReq, fileClass *dvCom.RepoFilesClass, maxFileSize int64) error {
	getRepoFileTree := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.RepoName,
		Ref:       req.Ref,
		Path:      req.Folder,
		RepoType:  req.RepoType,
	}
	gitFiles, err := req.GSTree(context.Background(), getRepoFileTree)
	if err != nil {
		return fmt.Errorf("get repo file tree error: %w", err)
	}
	for _, file := range gitFiles {
		if file.Type == "dir" {
			err := getAllFiles(dvCom.RepoFilesReq{
				Namespace: req.Namespace,
				RepoName:  req.RepoName,
				Folder:    file.Path,
				RepoType:  req.RepoType,
				Ref:       req.Ref,
				GSTree:    req.GSTree,
			}, fileClass, maxFileSize)
			if err != nil {
				return fmt.Errorf("list folder %s files error: %w", file.Path, err)
			}
		} else {
			appendFile(file, fileClass, maxFileSize)
		}
	}
	return nil
}

func appendFile(file *types.File, fileClass *dvCom.RepoFilesClass, maxFileSize int64) {
	fileClass.AllFiles[file.Path] = file
	if IsValidParquetFile(file.Name) {
		fileClass.ParquetFiles[file.Path] = file
	} else if IsValidJsonFile(file.Name) {
		if file.Size <= maxFileSize {
			fileClass.JsonlFiles[file.Path] = file
		}
	} else if IsValidCSVFile(file.Name) {
		if file.Size <= maxFileSize {
			fileClass.CsvFiles[file.Path] = file
		}
	}
}
