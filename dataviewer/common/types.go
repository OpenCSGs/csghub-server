package common

import (
	"context"
	"regexp"

	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type RepoDataType string

var (
	WILDCARD                         = "*"
	REG                              = regexp.MustCompile(`(?s)---\n(.*?)\n---`)
	ParquetBranch                    = "refs-convert-parquet"
	DuckdbBranch                     = "refs-convert-duckdb"
	TaskQueueDataViewerDatasetUpdate = "data_viewer_queue_dataset_update"
)

type ViewParquetFileReq struct {
	Namespace   string `json:"namespace"`
	RepoName    string `json:"name"`
	Branch      string `json:"branch"`
	Path        string `json:"path"`
	Per         int    `json:"per"`
	Page        int    `json:"page"`
	CurrentUser string `json:"current_user"`
	RepoID      int64  `json:"repo_id"`
	Migrated    bool   `json:"migrated"`
}

type ViewParquetFileResp struct {
	Columns     []string        `json:"columns"`
	ColumnsType []string        `json:"columns_type"`
	Rows        [][]interface{} `json:"rows"`
	Total       int             `json:"total"`
	Orderby     string          `json:"orderby"`
	Where       string          `json:"where"`
	Search      string          `json:"search"`
}

type CardData struct {
	Configs         []ConfigData  `yaml:"configs" json:"configs"`
	DatasetInfos    []DatasetInfo `yaml:"dataset_info" json:"dataset_info"`
	Downloaded_Size int64         `yaml:"downloaded_size" json:"downloaded_size"`
	Converted_Size  int64         `yaml:"converted_size" json:"converted_size"`
	Rows_Num        int64         `yaml:"rows_num" json:"rows_num"`
}

type ConfigData struct {
	ConfigName string      `yaml:"config_name" json:"config_name"`
	DataFiles  []DataFiles `yaml:"data_files" json:"data_files"`
}

type DataFiles struct {
	Split string      `yaml:"split" json:"split"`
	Path  interface{} `yaml:"path" json:"path"`
}

type DatasetInfo struct {
	ConfigName string  `yaml:"config_name" json:"config_name"`
	Splits     []Split `yaml:"splits" json:"splits"`
}

type Split struct {
	Name        string       `yaml:"name" json:"name"`
	NumExamples int          `yaml:"num_examples" json:"num_examples"`
	Files       []FileObject `yaml:"files,omitempty" json:"files,omitempty"`
	Origins     []FileObject `yaml:"origins,omitempty" json:"origins,omitempty"`
}

type RepoFilesReq struct {
	Namespace      string
	RepoName       string
	RepoType       types.RepositoryType
	Ref            string
	Folder         string
	GSTree         func(ctx context.Context, req gitserver.GetRepoInfoByPathReq) ([]*types.File, error)
	TotalLimitSize int64
}

type RepoFile struct {
	*types.File
	DownloadSize int64
}

type RepoFilesClass struct {
	AllFiles      map[string]*RepoFile
	ParquetFiles  map[string]*RepoFile
	JsonlFiles    map[string]*RepoFile
	CsvFiles      map[string]*RepoFile
	TotalJsonSize int64
	TotalCsvSize  int64
}

type DownloadCard struct {
	Configs []ConfigData     `yaml:"configs" json:"configs"`
	Subsets []DownloadSubset `yaml:"subsets" json:"subsets"`
}

type DownloadSubset struct {
	ConfigName string          `yaml:"config_name" json:"config_name"`
	Splits     []DownloadSplit `yaml:"splits" json:"splits"`
}

type DownloadSplit struct {
	Name       string       `yaml:"name" json:"name"`
	LocalPath  string       `yaml:"local_path" json:"local_path"`
	ExportPath string       `yaml:"export_path" json:"export_path"`
	Files      []FileObject `yaml:"files,omitempty" json:"files,omitempty"`
}

type FileObject struct {
	RepoFile        string `yaml:"repo_file" json:"repo_file"`
	Size            int64  `yaml:"size" json:"size"`
	LastCommit      string `yaml:"last_commit" json:"last_commit"`
	Lfs             bool   `yaml:"lfs" json:"lfs"`
	LfsRelativePath string `yaml:"lfs_relative_path" json:"lfs_relative_path"`
	SubsetName      string `yaml:"subset_name" json:"subset_name"`
	SplitName       string `yaml:"split_name" json:"split_name"`
	ConvertPath     string `yaml:"convert_path" json:"convert_path"`
	ObjectKey       string `yaml:"object_key" json:"object_key"`
	LocalRepoPath   string `yaml:"local_repo_path" json:"local_repo_path"`
	LocalFileName   string `yaml:"local_file_name" json:"local_file_name"`
	DownloadSize    int64  `yaml:"download_size" json:"download_size"`
	LfsSHA256       string `yaml:"lfs_sha256" json:"lfs_sha256"`
}

type CataLogRespone struct {
	Configs         []ConfigData  `yaml:"configs" json:"configs"`
	DatasetInfos    []DatasetInfo `yaml:"dataset_info" json:"dataset_info"`
	Status          int           `yaml:"status" json:"status"`
	Logs            string        `yaml:"logs" json:"logs"`
	Downloaded_Size int64         `yaml:"downloaded_size" json:"downloaded_size"`
	Converted_Size  int64         `yaml:"converted_size" json:"converted_size"`
	Rows_Num        int64         `yaml:"rows_num" json:"rows_num"`
}

type WorkflowUpdateParams struct {
	Req    types.UpdateViewerReq
	Config *config.Config
}

type ScanRepoFileReq struct {
	Req              types.UpdateViewerReq
	ConvertLimitSize int64
}

type DetermineCardReq struct {
	Card         CardData
	Class        RepoFilesClass
	RepoDataType RepoDataType
}

type CheckBuildReq struct {
	Req  types.UpdateViewerReq
	Card CardData
}

type CopyParquetReq struct {
	Req              types.UpdateViewerReq
	ComputedCardData CardData
	NewBranch        string
}

type DownloadFileReq CheckBuildReq

type ConvertReq struct {
	Req          types.UpdateViewerReq
	DownloadCard DownloadCard
	RepoDataType RepoDataType
}

type UploadParquetReq struct {
	Req          types.UpdateViewerReq
	DownloadCard DownloadCard
	NewBranch    string
}

type UpdateCardReq struct {
	Req            types.UpdateViewerReq
	OriginCardData CardData
	FinalCardData  CardData
}

type UpdateWorkflowStatusReq struct {
	Req                types.UpdateViewerReq
	WorkflowErrMsg     string
	ShouldUpdateViewer bool
}

type FileExtName struct {
	Parquet string
	Jsonl   string
	Json    string
	Csv     string
}

type SplitName struct {
	Train string
	Test  string
	Val   string
	Other string
}
