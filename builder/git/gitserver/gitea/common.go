package gitea

import (
	"context"

	"opencsg.com/csghub-server/common/types"
)

func (c *Client) BuildRelativePath(ctx context.Context, repoType types.RepositoryType, namespace, name string) (string, error) {
	return "", nil
}
