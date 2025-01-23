package workflows

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"go.temporal.io/sdk/activity"
	"opencsg.com/csghub-server/common/types"
	dvCom "opencsg.com/csghub-server/dataviewer/common"
)

func TransferFileObject(file *types.File, subsetName, splitName string) dvCom.FileObject {
	return dvCom.FileObject{
		RepoFile:        file.Path,
		Size:            file.Size,
		LastCommit:      file.LastCommitSHA,
		Lfs:             file.Lfs,
		LfsRelativePath: file.LfsRelativePath,
		SubsetName:      subsetName,
		SplitName:       splitName,
	}
}

func GetPatternFileList(path interface{}) []string {
	if path == nil {
		return []string{}
	}
	var (
		files []string
	)
	if slice, ok := path.([]interface{}); ok {
		for _, v := range slice {
			files = append(files, v.(string))
		}
	} else if slice, ok := path.([]string); ok {
		files = slice
	} else {
		files = []string{path.(string)}
	}
	return files
}

func ConvertRealFiles(splitFiles []string, sortKeys []string, targetFiles map[string]*types.File, subsetName, splitName string) []dvCom.FileObject {
	var phyFiles []dvCom.FileObject
	for _, filePattern := range splitFiles {
		if !strings.Contains(filePattern, dvCom.WILDCARD) {
			file, exists := targetFiles[filePattern]
			if exists {
				phyFiles = append(phyFiles, TransferFileObject(file, subsetName, splitName))
			}
			continue
		}

		fileReg, err := regexp.Compile(filePattern)
		if err != nil {
			slog.Warn("invalid regexp format of split file", slog.Any("filePattern", filePattern), slog.Any("err", err))
			file, exists := targetFiles[filePattern]
			if exists {
				phyFiles = append(phyFiles, TransferFileObject(file, subsetName, splitName))
			}
			continue
		}
		for _, path := range sortKeys {
			// repo file match like: test/test-*
			if fileReg.MatchString(path) {
				file, exists := targetFiles[path]
				if exists {
					phyFiles = append(phyFiles, TransferFileObject(file, subsetName, splitName))
				}
			}
		}
	}
	slog.Debug("convert real files", slog.Any("splitFiles", splitFiles), slog.Any("phyFiles", phyFiles))
	return phyFiles
}

func GetCardDataMD5(finalCardData dvCom.CardData) string {
	hasher := md5.New()
	for _, info := range finalCardData.DatasetInfos {
		for _, split := range info.Splits {
			for _, file := range split.Origins {
				hasher.Write([]byte(fmt.Sprintf("%s-%s", file.RepoFile, file.LastCommit)))
			}
		}
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func GenerateWorkFlowID(req types.UpdateViewerReq) string {
	id := "dv-" + req.Namespace + "/" + req.Name + "-" + time.Now().Format("20060102.150405.000000000")
	return id
}

func GetCacheRepoPath(ctx context.Context, cacheDir string, req types.UpdateViewerReq) string {
	wfCtx := activity.GetInfo(ctx)
	workflowID := wfCtx.WorkflowExecution.ID
	segments := strings.Split(workflowID, "-")
	timeSeq := segments[len(segments)-1]
	return fmt.Sprintf("%s/%s/%s/%s/%s", strings.TrimSuffix(cacheDir, "/"), req.RepoType, req.Namespace, req.Name, timeSeq)
}
