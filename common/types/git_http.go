package types

import (
	"errors"
	"net/http"
	"path"
	"regexp"
	"time"

	pb "gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
)

const LfsMediaType = "application/vnd.git-lfs+json"

var (
	oidPattern      = regexp.MustCompile(`^[a-f\d]{64}$`)
	ErrHashMismatch = errors.New("content hash does not match OID")
	ErrSizeMismatch = errors.New("content size does not match")
)

type InfoRefsReq struct {
	Namespace   string         `json:"namespace"`
	Name        string         `json:"name"`
	RepoType    RepositoryType `json:"repo_type"`
	Rpc         string         `json:"rpc"`
	GitProtocol string         `json:"git_protocol"`
	CurrentUser string         `json:"current_user"`
}

type GitUploadPackReq struct {
	Namespace   string              `json:"namespace"`
	Name        string              `json:"name"`
	RepoType    RepositoryType      `json:"repo_type"`
	GitProtocol string              `json:"git_protocol"`
	Request     *http.Request       `json:"request"`
	Writer      http.ResponseWriter `json:"writer"`
	CurrentUser string              `json:"current_user"`
}

type GitReceivePackReq = GitUploadPackReq

type BatchRequest struct {
	Operation     string         `json:"operation"`
	Transfers     []string       `json:"transfers,omitempty"`
	Ref           *Reference     `json:"ref,omitempty"`
	Objects       []Pointer      `json:"objects"`
	Authorization string         `json:"authorization"`
	Namespace     string         `json:"namespace"`
	Name          string         `json:"name"`
	RepoType      RepositoryType `json:"repo_type"`
	CurrentUser   string         `json:"current_user"`
}

type UploadRequest struct {
	Oid         string         `json:"oid"`
	Size        int64          `json:"size"`
	CurrentUser string         `json:"current_user"`
	Namespace   string         `json:"namespace"`
	Name        string         `json:"name"`
	RepoType    RepositoryType `json:"repo_type"`
}

type DownloadRequest struct {
	Oid         string         `json:"oid"`
	Size        int64          `json:"size"`
	CurrentUser string         `json:"current_user"`
	Namespace   string         `json:"namespace"`
	Name        string         `json:"name"`
	RepoType    RepositoryType `json:"repo_type"`
	SaveAs      string         `json:"save_as"`
}

type VerifyRequest struct {
	CurrentUser string         `json:"current_user"`
	Namespace   string         `json:"namespace"`
	Name        string         `json:"name"`
	RepoType    RepositoryType `json:"repo_type"`
}

type Reference struct {
	Name string `json:"name"`
}

type Pointer struct {
	Oid         string `json:"oid"`
	Size        int64  `json:"size"`
	DownloadURL string `json:"download_url"`
}

type BatchResponse struct {
	Transfer string            `json:"transfer,omitempty"`
	Objects  []*ObjectResponse `json:"objects"`
}

type Link struct {
	Href      string            `json:"href"`
	Header    map[string]string `json:"header,omitempty"`
	ExpiresAt *time.Time        `json:"expires_at,omitempty"`
}

type ObjectResponse struct {
	Pointer
	Actions map[string]*Link `json:"actions,omitempty"`
	Error   *ObjectError     `json:"error,omitempty"`
}

type ObjectError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type SSHAllowedReq struct {
	Namespace     string         `json:"namespace"`
	Name          string         `json:"name"`
	RepoType      RepositoryType `json:"repo_type"`
	Action        string         `json:"action"`
	Repo          string         `json:"project"`
	Changes       string         `json:"changes"`
	Protocol      string         `json:"protocol"`
	KeyID         string         `json:"key_id,omitempty"`
	Username      string         `json:"username,omitempty"`
	Krb5Principal string         `json:"krb5principal,omitempty"`
	CheckIP       string         `json:"check_ip,omitempty"`
	NamespacePath string         `json:"namespace_path,omitempty"`
}

type SSHAllowedResp struct {
	Success          bool          `json:"status"`
	Message          string        `json:"message"`
	Repo             string        `json:"gl_repository"`
	UserID           string        `json:"gl_id"`
	KeyType          string        `json:"gl_key_type"`
	KeyID            int           `json:"gl_key_id"`
	ProjectID        int           `json:"gl_project_id"`
	RootNamespaceID  int           `json:"gl_root_namespace_id"`
	Username         string        `json:"gl_username"`
	GitConfigOptions []string      `json:"git_config_options"`
	Gitaly           Gitaly        `json:"gitaly"`
	GitProtocol      string        `json:"git_protocol"`
	Payload          CustomPayload `json:"payload"`
	ConsoleMessages  []string      `json:"gl_console_messages"`
	Who              string
	StatusCode       int
	// NeedAudit indicates whether git event should be audited to rails.
	NeedAudit bool `json:"need_audit"`
}

type CustomPayload struct {
	Action string            `json:"action"`
	Data   CustomPayloadData `json:"data"`
}

type CustomPayloadData struct {
	APIEndpoints                            []string          `json:"api_endpoints"`
	Username                                string            `json:"gl_username"`
	PrimaryRepo                             string            `json:"primary_repo"`
	UserID                                  string            `json:"gl_id,omitempty"`
	RequestHeaders                          map[string]string `json:"request_headers"`
	GeoProxyDirectToPrimary                 bool              `json:"geo_proxy_direct_to_primary"`
	GeoProxyFetchDirectToPrimary            bool              `json:"geo_proxy_fetch_direct_to_primary"`
	GeoProxyFetchDirectToPrimaryWithOptions bool              `json:"geo_proxy_fetch_direct_to_primary_with_options"`
	GeoProxyFetchSSHDirectToPrimary         bool              `json:"geo_proxy_fetch_ssh_direct_to_primary"`
	GeoProxyPushSSHDirectToPrimary          bool              `json:"geo_proxy_push_ssh_direct_to_primary"`
}

type Gitaly struct {
	Repo     pb.Repository     `json:"repository"`
	Address  string            `json:"address"`
	Token    string            `json:"token"`
	Features map[string]string `json:"features"`
}

type LfsAuthenticateReq struct {
	Namespace string         `json:"namespace"`
	Name      string         `json:"name"`
	RepoType  RepositoryType `json:"repo_type"`
	Operation string         `json:"operation"`
	Repo      string         `json:"project"`
	KeyID     string         `json:"key_id,omitempty"`
	UserID    string         `json:"user_id,omitempty"`
}

type LfsAuthenticateResp struct {
	Username  string `json:"username"`
	LfsToken  string `json:"lfs_token"`
	RepoPath  string `json:"repository_http_path"`
	ExpiresIn int    `json:"expires_in"`
}

type GitalyAllowedReq struct {
	Action       string `json:"action"`
	GlRepository string `json:"gl_repository"`
	Project      string `json:"project"`
	Changes      string `json:"changes"`
	Protocol     string `json:"protocol"`
	Env          string `json:"env"`
	UserID       string `json:"user_id"`
	KeyID        string `json:"key_id"`
	CheckIP      string `json:"check_ip"`
}

func (p Pointer) Valid() bool {
	if len(p.Oid) != 64 {
		return false
	}
	if !oidPattern.MatchString(p.Oid) {
		return false
	}
	if p.Size < 0 {
		return false
	}
	return true
}

func (p Pointer) RelativePath() string {
	if len(p.Oid) < 5 {
		return p.Oid
	}

	return path.Join(p.Oid[0:2], p.Oid[2:4], p.Oid[4:])
}
