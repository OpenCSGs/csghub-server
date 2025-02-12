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
	RepoParquetData   dvCom.RepoDataType = "parquet"
	RepoJsonData      dvCom.RepoDataType = "json"
	RepoCsvData       dvCom.RepoDataType = "csv"
	DataSizePerThread                    = 804857600
)

var (
	GitDefaultUserName  = "admin"
	GitDefaultUserEmail = "admin@csghub.com"
	DefaultSubsetName   = "default"
	SplitName           = dvCom.SplitName{
		Train: "train",
		Test:  "test",
		Val:   "validation",
		Other: "others",
	}
	FileExtName = dvCom.FileExtName{
		Parquet: ".parquet",
		Jsonl:   ".jsonl",
		Json:    ".json",
		Csv:     ".csv",
	}
	MinFileSizeGap = int64(1048576)
)

func IsValidParquetFile(entryName string) bool {
	return strings.HasSuffix(strings.ToLower(entryName), FileExtName.Parquet)
}

func IsValidJsonFile(entryName string) bool {
	return strings.HasSuffix(strings.ToLower(entryName), FileExtName.Jsonl) ||
		strings.HasSuffix(strings.ToLower(entryName), FileExtName.Json)
}

func IsValidCSVFile(entryName string) bool {
	return strings.HasSuffix(strings.ToLower(entryName), FileExtName.Csv)
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

func GetFilePaths(req dvCom.RepoFilesReq, fileClass *dvCom.RepoFilesClass) error {
	err := getAllFiles(req, fileClass)
	if err != nil {
		return fmt.Errorf("get repo all files error: %w", err)
	}
	return nil
}

func getAllFiles(req dvCom.RepoFilesReq, fileClass *dvCom.RepoFilesClass) error {
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
				Namespace:      req.Namespace,
				RepoName:       req.RepoName,
				Folder:         file.Path,
				RepoType:       req.RepoType,
				Ref:            req.Ref,
				GSTree:         req.GSTree,
				TotalLimitSize: req.TotalLimitSize,
			}, fileClass)
			if err != nil {
				return fmt.Errorf("list folder %s files error: %w", file.Path, err)
			}
		} else {
			appendFile(file, fileClass, req.TotalLimitSize)
		}
	}
	return nil
}

func appendFile(file *types.File, fileClass *dvCom.RepoFilesClass, limitSize int64) {
	repoFile := &dvCom.RepoFile{
		File:         file,
		DownloadSize: file.Size,
	}
	fileClass.AllFiles[file.Path] = repoFile

	if IsValidParquetFile(file.Name) {
		fileClass.ParquetFiles[file.Path] = repoFile
		return
	}

	if IsValidJsonFile(file.Name) {
		if fileClass.TotalJsonSize >= limitSize {
			return
		}
		if fileClass.TotalJsonSize+file.Size > limitSize {
			fileClass.JsonlFiles[file.Path] = &dvCom.RepoFile{
				File:         file,
				DownloadSize: limitSize - fileClass.TotalJsonSize,
			}
		} else {
			fileClass.JsonlFiles[file.Path] = repoFile
		}
		fileClass.TotalJsonSize += fileClass.JsonlFiles[file.Path].DownloadSize
		return
	}

	if IsValidCSVFile(file.Name) {
		if fileClass.TotalCsvSize >= limitSize {
			return
		}
		if fileClass.TotalCsvSize+file.Size > limitSize {
			fileClass.CsvFiles[file.Path] = &dvCom.RepoFile{
				File:         file,
				DownloadSize: limitSize - fileClass.TotalCsvSize,
			}
		} else {
			fileClass.CsvFiles[file.Path] = repoFile
		}
		fileClass.TotalCsvSize += fileClass.CsvFiles[file.Path].DownloadSize
		return
	}

}
