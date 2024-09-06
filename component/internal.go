package component

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"

	pb "gitlab.com/gitlab-org/gitaly/v16/proto/go/gitalypb"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/gitserver/gitaly"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

type InternalComponent struct {
	config      *config.Config
	sshKeyStore *database.SSHKeyStore
	repoStore   *database.RepoStore
	*RepoComponent
}

func NewInternalComponent(config *config.Config) (*InternalComponent, error) {
	var err error
	c := &InternalComponent{}
	c.config = config
	c.sshKeyStore = database.NewSSHKeyStore()
	c.repoStore = database.NewRepoStore()
	c.RepoComponent, err = NewRepoComponent(config)
	c.tokenStore = database.NewAccessTokenStore()
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *InternalComponent) Allowed(ctx context.Context) (bool, error) {
	return true, nil
}

func (c *InternalComponent) SSHAllowed(ctx context.Context, req types.SSHAllowedReq) (*types.SSHAllowedResp, error) {
	namespace, err := c.namespace.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to find namespace %s: %v", req.Namespace, err)
	}
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, err: %v", err)
	}
	if repo == nil {
		return nil, errors.New("repo not found")
	}
	keyId, err := strconv.ParseInt(req.KeyID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse key ID, err: %v", err)
	}
	sshKey, err := c.sshKeyStore.FindByID(ctx, keyId)
	if err != nil {
		return nil, fmt.Errorf("failed to find ssh key by id, err: %v", err)
	}
	if req.Action == "git-receive-pack" {
		allowed, err := c.AllowWriteAccess(ctx, req.RepoType, req.Namespace, req.Name, sshKey.User.Username)
		if err != nil {
			return nil, ErrUnauthorized
		}
		if !allowed {
			return nil, ErrForbidden
		}
	} else if req.Action == "git-upload-pack" {
		if repo.Private {
			allowed, err := c.AllowReadAccess(ctx, req.RepoType, req.Namespace, req.Name, sshKey.User.Username)
			if err != nil {
				return nil, ErrUnauthorized
			}
			if !allowed {
				return nil, ErrForbidden
			}
		}
	}
	repoType := fmt.Sprintf("%ss", string(req.RepoType))

	return &types.SSHAllowedResp{
		Success:          true,
		Message:          "allowed",
		Repo:             req.Repo,
		UserID:           strconv.FormatInt(sshKey.User.ID, 10),
		KeyType:          "ssh",
		KeyID:            int(sshKey.ID),
		ProjectID:        int(repo.ID),
		RootNamespaceID:  int(namespace.ID),
		GitConfigOptions: []string{"uploadpack.allowFilter=true", "uploadpack.allowAnySHA1InWant=true"},
		Gitaly: types.Gitaly{
			Repo: pb.Repository{
				StorageName:  c.config.GitalyServer.Storge,
				RelativePath: gitaly.BuildRelativePath(repoType, req.Namespace, req.Name),
				GlRepository: filepath.Join(repoType, req.Namespace, req.Name),
			},
			Address: c.config.GitalyServer.Address,
			Token:   c.config.GitalyServer.Token,
		},
		StatusCode: 200,
	}, nil
}

func (c *InternalComponent) GetAuthorizedKeys(ctx context.Context, key string) (*database.SSHKey, error) {
	fingerprint, err := common.CalculateAuthorizedSSHKeyFingerprint(key)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate authorized keys fingerprint, error: %v", err)
	}
	sshKey, err := c.sshKeyStore.FindByFingerpringSHA256(ctx, fingerprint)
	if err != nil {
		return nil, fmt.Errorf("failed to get authorized keys, error: %v", err)
	}
	return sshKey, nil
}

func (c *InternalComponent) GetCommitDiff(ctx context.Context, req types.GetDiffBetweenTwoCommitsReq) (*types.GiteaCallbackPushReq, error) {
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, err: %v", err)
	}
	if repo == nil {
		return nil, errors.New("repo not found")
	}
	diffs, err := c.git.GetDiffBetweenTwoCommits(ctx, gitserver.GetDiffBetweenTwoCommitsReq{
		Namespace:     req.Namespace,
		Name:          req.Name,
		RepoType:      req.RepoType,
		Ref:           req.Ref,
		LeftCommitId:  req.LeftCommitId,
		RightCommitId: req.RightCommitId,
		Private:       repo.Private,
	})
	if err != nil {
		return nil, err
	}
	return diffs, nil
}

func (c *InternalComponent) LfsAuthenticate(ctx context.Context, req types.LfsAuthenticateReq) (*types.LfsAuthenticateResp, error) {
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, err: %v", err)
	}
	if repo == nil {
		return nil, errors.New("repo not found")
	}
	keyId, err := strconv.ParseInt(req.KeyID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse key ID, err: %v", err)
	}
	sshKey, err := c.sshKeyStore.FindByID(ctx, keyId)
	if err != nil {
		return nil, fmt.Errorf("failed to find ssh key by id, err: %v", err)
	}
	if repo.Private {
		allowed, err := c.AllowReadAccess(ctx, req.RepoType, req.Namespace, req.Name, sshKey.User.Username)
		if err != nil {
			return nil, ErrUnauthorized
		}
		if !allowed {
			return nil, ErrForbidden
		}
	}
	token, err := c.tokenStore.GetUserGitToken(ctx, sshKey.User.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to find git token by username, err: %v", err)
	}
	repoType := fmt.Sprintf("%ss", string(req.RepoType))
	return &types.LfsAuthenticateResp{
		Username: sshKey.User.Username,
		LfsToken: token.Token,
		RepoPath: c.config.APIServer.PublicDomain + "/" + filepath.Join(repoType, req.Namespace, req.Name+".git"),
	}, nil
}

