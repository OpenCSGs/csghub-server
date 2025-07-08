package gitaly

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

func (c *Client) BuildRelativePath(ctx context.Context, repoType types.RepositoryType, namespace, name string) (string, error) {
	repo, err := c.repoStore.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return "", fmt.Errorf("failed to find repository: %w", err)
	}
	if repo.Hashed {
		return common.BuildHashedRelativePath(repo.ID) + ".git", nil
	}
	repoTypeS := fmt.Sprintf("%ss", string(repoType))
	return common.BuildRelativePath(repoTypeS, namespace, name) + ".git", nil
}
