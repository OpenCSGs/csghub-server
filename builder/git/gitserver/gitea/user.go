package gitea

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"math/big"

	"github.com/OpenCSGs/gitea-go-sdk/gitea"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

func (c *Client) CreateUser(u gitserver.CreateUserRequest) (user *gitserver.CreateUserResponse, err error) {
	password, err := generateRandomPassword(12)
	if err != nil {
		return
	}

	giteaUser, _, err := c.giteaClient.AdminCreateUser(
		gitea.CreateUserOption{
			FullName:           u.Nickname,
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

	err = c.createOrgsForUser(giteaUser)

	if err != nil {
		return
	}

	password = calculateSHA1(password)
	user = &gitserver.CreateUserResponse{
		Email:    giteaUser.Email,
		GitID:    giteaUser.ID,
		Username: giteaUser.UserName,
		NickName: giteaUser.FullName,
		Password: password,
	}

	return
}

func (c *Client) UpdateUser(u *types.UpdateUserRequest, user *database.User) (*database.User, error) {
	_, err := c.giteaClient.AdminEditUser(
		u.Username,
		gitea.EditUserOption{
			LoginName: u.Username,
			FullName:  u.Nickname,
			Email:     u.Email,
		},
	)

	user.NickName = *u.Nickname
	user.Email = *u.Email
	return user, err
}

func (c *Client) UpdateUserV2(u gitserver.UpdateUserRequest) error {
	//nothing to update
	if u.Nickname == nil && u.Email == nil {
		return nil
	}

	opt := gitea.EditUserOption{
		LoginName: u.Username,
	}
	if u.Nickname != nil {
		opt.FullName = u.Nickname
	}

	if u.Email != nil {
		opt.Email = u.Email
	}
	_, err := c.giteaClient.AdminEditUser(
		u.Username,
		opt,
	)

	return err
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

// Create three orgs for user
func (c *Client) createOrgsForUser(user *gitea.User) (err error) {
	orgNames := []string{
		common.WithPrefix(user.UserName, ModelOrgPrefix),
		common.WithPrefix(user.UserName, DatasetOrgPrefix),
		common.WithPrefix(user.UserName, SpaceOrgPrefix),
		common.WithPrefix(user.UserName, CodeOrgPrefix),
	}

	for _, orgName := range orgNames {
		_, _, err = c.giteaClient.AdminCreateOrg(
			user.UserName,
			gitea.CreateOrgOption{
				Name:     orgName,
				FullName: orgName,
			},
		)
		if err != nil {
			return
		}
	}

	return
}

// Create gitea orgs for user to store different type repositories
func (c *Client) FixUserData(ctx context.Context, userName string) (err error) {
	orgNames := []string{
		common.WithPrefix(userName, ModelOrgPrefix),
		common.WithPrefix(userName, DatasetOrgPrefix),
		common.WithPrefix(userName, SpaceOrgPrefix),
		common.WithPrefix(userName, CodeOrgPrefix),
	}

	for _, orgName := range orgNames {
		_, _, err = c.giteaClient.AdminCreateOrg(
			userName,
			gitea.CreateOrgOption{
				Name:     orgName,
				FullName: orgName,
			},
		)
	}

	return
}

func calculateSHA1(input string) string {
	hasher := sha1.New()
	hasher.Write([]byte(input))
	hashInBytes := hasher.Sum(nil)
	hashString := hex.EncodeToString(hashInBytes)

	return hashString
}
