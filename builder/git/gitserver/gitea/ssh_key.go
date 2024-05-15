package gitea

import (
	"github.com/OpenCSGs/gitea-go-sdk/gitea"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func (c *Client) CreateSSHKey(req *types.CreateSSHKeyRequest) (token *database.SSHKey, err error) {
	giteaSSHKey, _, err := c.giteaClient.AdminCreateUserPublicKey(
		req.Username,
		gitea.CreateKeyOption{
			Title: req.Name,
			Key:   req.Content,
		},
	)

	if err != nil {
		return
	}

	token = &database.SSHKey{
		GitID:   giteaSSHKey.ID,
		Name:    req.Name,
		Content: giteaSSHKey.Key,
	}

	return
}

//List all SSH keys from gitea

// func (c *Client) ListSSHKeys(username string, per, page int) (tokens []*database.SSHKey, err error) {
// 	giteaSSHKeys, _, err := c.giteaClient.ListPublicKeys(
// 		username,
// 		gitea.ListPublicKeysOptions{
// 			ListOptions: gitea.ListOptions{
// 				Page:     page,
// 				PageSize: per,
// 			},
// 		},
// 	)

// 	if err != nil {
// 		return
// 	}

// 	for _, giteaSSHKey := range giteaSSHKeys {
// 		tokens = append(tokens, &database.SSHKey{
// 			GID:     int(giteaSSHKey.ID),
// 			Name:    giteaSSHKey.Title,
// 			Content: giteaSSHKey.Key,
// 		})
// 	}

// 	return
// }

func (c *Client) DeleteSSHKey(id int) (err error) {
	_, err = c.giteaClient.DeletePublicKey(int64(id))
	return
}
