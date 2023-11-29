package gitea

import (
	"crypto/rand"
	"math/big"

	"git-devops.opencsg.com/product/community/starhub-server/pkg/store/database"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/types"
	"github.com/pulltheflower/gitea-go-sdk/gitea"
)

func (c *Client) CreateUser(u *types.CreateUserRequest) (user *database.User, err error) {
	password, err := generateRandomPassword(12)
	if err != nil {
		return
	}

	giteaUser, _, err := c.giteaClient.AdminCreateUser(
		gitea.CreateUserOption{
			FullName:           u.Name,
			Username:           u.Username,
			Email:              u.Email,
			Password:           password,
			MustChangePassword: gitea.OptionalBool(false),
			SendNotify:         false,
		},
	)

	if err != nil {
		return
	}

	user = &database.User{
		Email:    giteaUser.Email,
		GitID:    int(giteaUser.ID),
		Username: giteaUser.UserName,
		Name:     giteaUser.FullName,
		Password: password,
	}

	return
}

func (c *Client) UpdateUser(u *types.UpdateUserRequest) (int, *database.User, error) {
	resp, err := c.giteaClient.AdminEditUser(
		u.Username,
		gitea.EditUserOption{
			LoginName: u.Username,
			FullName:  gitea.OptionalString(u.Name),
			Email:     gitea.OptionalString(u.Email),
		},
	)
	user := &database.User{
		Username: u.Username,
		Name:     u.Name,
		Email:    u.Email,
	}
	return resp.StatusCode, user, err
}

// Random password generator
func generateRandomPassword(length int) (string, error) {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()_+"
	charsetLength := big.NewInt(int64(len(charset)))
	password := make([]byte, length)

	for i := range password {
		randomIndex, err := rand.Int(rand.Reader, charsetLength)
		if err != nil {
			return "", err
		}
		password[i] = charset[randomIndex.Int64()]
	}

	return string(password), nil
}
