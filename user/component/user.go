package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
	"github.com/google/uuid"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

const GitalyRepoNotFoundErr = "rpc error: code = NotFound desc = repository does not exist"

type userComponentImpl struct {
	userStore database.UserStore
	orgStore  database.OrgStore
	nsStore   database.NamespaceStore
	repo      database.RepoStore
	ds        database.DeployTaskStore
	ams       database.AccountMeteringStore

	gs     gitserver.GitServer
	jwtc   JwtComponent
	tokenc AccessTokenComponent

	casc      *casdoorsdk.Client
	casConfig *casdoorsdk.AuthConfig
	once      *sync.Once
	sfnode    *snowflake.Node
	config    *config.Config
}

type UserComponent interface {
	ChangeUserName(ctx context.Context, oldUserName, newUserName, opUser string) error
	UpdateByUUID(ctx context.Context, req *types.UpdateUserRequest) error
	Update(ctx context.Context, req *types.UpdateUserRequest, opUser string) error
	Delete(ctx context.Context, operator, username string) error
	// CanAdmin checks if a user has admin privileges.
	//
	// Parameters:
	// - ctx: The context.Context object for the function.
	// - username: The username of the user to check.
	//
	// Returns:
	// - bool: True if the user has admin privileges, false otherwise.
	// - error: An error if the user cannot be found in the database.
	CanAdmin(ctx context.Context, username string) (bool, error)
	// GetInternal get *full* user info by username or uuid
	//
	// should only be called by other *internal* services
	GetInternal(ctx context.Context, userNameOrUUID string, useUUID bool) (*types.User, error)
	Get(ctx context.Context, userNameOrUUID, visitorName string, useUUID bool) (*types.User, error)
	CheckOperatorAndUser(ctx context.Context, operator, username string) (bool, error)
	CheckIfUserHasOrgs(ctx context.Context, userName string) (bool, error)
	CheckIffUserHasRunningOrBuildingDeployments(ctx context.Context, userName string) (bool, error)
	CheckIfUserHasBills(ctx context.Context, userName string) (bool, error)
	Index(ctx context.Context, visitorName, search string, per, page int) ([]*types.User, int, error)
	Signin(ctx context.Context, code, state string) (*types.JWTClaims, string, error)
	FixUserData(ctx context.Context, userName string) error
}

func NewUserComponent(config *config.Config) (UserComponent, error) {
	var err error
	c := &userComponentImpl{}
	c.userStore = database.NewUserStore()
	c.orgStore = database.NewOrgStore()
	c.nsStore = database.NewNamespaceStore()
	c.repo = database.NewRepoStore()
	c.ds = database.NewDeployTaskStore()
	c.ams = database.NewAccountMeteringStore()
	c.jwtc = NewJwtComponent(config.JWT.SigningKey, config.JWT.ValidHour)
	c.tokenc, err = NewAccessTokenComponent(config)
	if err != nil {
		return nil, fmt.Errorf("fail to create access token component, error: %w", err)
	}
	c.gs, err = git.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("failed to create git server,error:%w", err)
		return nil, newError
	}
	c.once = new(sync.Once)

	certData, err := os.ReadFile(config.Casdoor.Certificate)
	if err != nil {
		return nil, fmt.Errorf("failed to read casdoor certificate file,error:%w", err)
	}
	c.casConfig = &casdoorsdk.AuthConfig{
		Endpoint:         config.Casdoor.Endpoint,
		ClientId:         config.Casdoor.ClientID,
		ClientSecret:     config.Casdoor.ClientSecret,
		Certificate:      string(certData),
		OrganizationName: config.Casdoor.OrganizationName,
		ApplicationName:  config.Casdoor.ApplicationName,
	}
	c.config = config
	return c, nil
}

// // This function creates a user when user register from portal, without casdoor
// func (c *userComponentImpl) createFromPortalRegistry(ctx context.Context, req types.CreateUserRequest) (*database.User, error) {
// 	// Panic if the function has not been implemented
// 	panic("implement me later")
// }

func (c *userComponentImpl) createFromCasdoorUser(ctx context.Context, cu casdoorsdk.User) (*database.User, error) {
	var (
		gsUserResp        *gitserver.CreateUserResponse
		err               error
		userName          string
		email             string
		canChangeUserName bool
	)
	//wechat user need to change username later
	if cu.WeChat != "" {
		userName, err = c.genUniqueName()
		if err != nil {
			return nil, fmt.Errorf("failed to generate unique user name,error:%w", err)
		}
		canChangeUserName = true
		//set email to "", make sure not to create git user
		email = ""
	} else {
		userName = cu.Name
		canChangeUserName = false
		email = cu.Email
	}
	//skip creating git user if email is empty, it will be created later when user set email
	if email != "" {
		gsUserReq := gitserver.CreateUserRequest{
			Nickname: userName,
			Username: userName,
			Email:    email,
		}
		gsUserResp, err = c.gs.CreateUser(gsUserReq)
		if err != nil {
			newError := fmt.Errorf("failed to create gitserver user '%s',error:%w", cu.Name, err)
			return nil, newError
		}
	}

	namespace := &database.Namespace{
		Path: userName,
	}
	user := &database.User{
		Username:    userName,
		NickName:    userName,
		Email:       email,
		UUID:        cu.Id,
		RegProvider: "casdoor",
		Gender:      cu.Gender,
		// RoleMask:        "", //will be updated when admin set user role
		Phone:           cu.Phone,
		PhoneVerified:   false,
		EmailVerified:   false,
		LastLoginAt:     cu.LastSigninTime,
		Avatar:          cu.Avatar,
		CompanyVerified: false,
		// PasswordHash:    cu.Password,
		Homepage:          cu.Homepage,
		Bio:               cu.Bio,
		CanChangeUserName: canChangeUserName,
	}
	if gsUserResp != nil {
		user.GitID = gsUserResp.GitID
		user.Password = gsUserResp.Password
	}
	err = c.userStore.Create(ctx, user, namespace)
	if err != nil {
		newError := fmt.Errorf("failed to create user in db,error:%w", err)
		return nil, newError
	}

	return user, nil
}

func (c *userComponentImpl) ChangeUserName(ctx context.Context, oldUserName, newUserName, opUser string) error {
	if oldUserName != opUser {
		return fmt.Errorf("user name can only be changed by user self, user: '%s', op user: '%s'", oldUserName, opUser)
	}

	user, err := c.userStore.FindByUsername(ctx, oldUserName)
	if err != nil {
		return fmt.Errorf("failed to find user by old name in db,error:%w", err)
	}

	if !user.CanChangeUserName {
		return fmt.Errorf("user name can not be changed")
	}

	newUser, err := c.userStore.FindByUsername(ctx, newUserName)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to find user by new name in db,error:%w", err)
	}
	if newUser.ID > 0 {
		return fmt.Errorf("user name '%s' already exists", newUserName)
	}

	err = c.userStore.ChangeUserName(ctx, oldUserName, newUserName)
	if err != nil {
		return fmt.Errorf("failed to change user name in db,error:%w", err)
	}

	//skip casdoor update if it's not a casdoor user
	if user.UUID == "" || user.RegProvider != "casdoor" {
		return nil
	}

	c.lazyInit()

	_, err = c.updateCasdoorUser(&types.UpdateUserRequest{
		UUID:        &user.UUID,
		NewUserName: &newUserName,
	})
	if err != nil {
		newError := fmt.Errorf("failed to update casdoor user, uuid:'%s',error:%w", user.UUID, err)
		return newError
	}
	return nil
}
func (c *userComponentImpl) UpdateByUUID(ctx context.Context, req *types.UpdateUserRequest) error {
	c.lazyInit()

	if req.UUID == nil {
		return errors.New("can not update user without uuid in request")
	}
	uuid := *req.UUID
	user, err := c.userStore.FindByUUID(ctx, uuid)
	if err != nil {
		return fmt.Errorf("failed to find user by uuid in db,error:%w", err)
	}
	opUserName := req.OpUser
	var opUser database.User
	if user.Username != opUserName {
		//find op user by username
		opUser, err = c.userStore.FindByUsername(ctx, opUserName)
		if err != nil {
			return fmt.Errorf("failed to find op user by name in db,user: '%s', error:%w", opUserName, err)
		}
	} else {
		opUser = *user
	}
	if req.Roles != nil {
		if can, reason := c.canChangeRole(*user, opUser); !can {
			return errors.New(reason)
		}
	}

	if req.NewUserName != nil && user.Username != *req.NewUserName {
		if can, reason := c.canChangeUserName(ctx, *user, opUser, *req.NewUserName); !can {
			return errors.New(reason)
		}
	}

	if req.Email != nil && user.Email != *req.Email {
		if can, reason := c.canChangeEmail(ctx, *user, opUser, *req.Email); !can {
			return errors.New(reason)
		}
	}

	if req.Phone != nil && user.Phone != *req.Phone {
		if can, reason := c.canChangePhone(*user, opUser, *req.Phone); !can {
			return errors.New(reason)
		}
	}

	// update user in casdoor first, then update user in db
	var oldCasdoorUser *casdoorsdk.User
	if user.RegProvider == "casdoor" {
		oldCasdoorUser, err = c.updateCasdoorUser(req)
		if err != nil {
			return fmt.Errorf("failed to update user in casdoor, uuid:'%s',error:%w", user.UUID, err)
		}
	}

	/* dont update git user email anymore, as gitea has been depricated */

	changedUser := c.setChangedProps(user, req)
	if err := c.userStore.Update(ctx, changedUser, user.Username); err != nil {
		// rollback casdoor user change
		// get id by user name before changed
		id := c.casc.GetId(oldCasdoorUser.Name)
		id = url.QueryEscape(id) // wechat user's name may contain special characters
		if _, err := c.casc.UpdateUserById(id, oldCasdoorUser); err != nil {
			slog.Error("failed to rollback casdoor user change", slog.String("uuid", user.UUID), slog.Any("error", err))
		}

		return fmt.Errorf("failed to update user in db,error:%w", err)
	}

	return nil

}

func (c *userComponentImpl) canChangeRole(user, opuser database.User) (can bool, reason string) {
	if opuser.ID == user.ID {
		return false, "user can not change roles of self"
	}
	if !opuser.CanAdmin() {
		return false, "op user is not admin"
	}
	return true, ""
}

func (c *userComponentImpl) canChangeUserName(ctx context.Context, user, opuser database.User, newUserName string) (can bool, reason string) {
	if opuser.ID != user.ID {
		return false, "user name can only be changed by the user itself"
	}
	if !user.CanChangeUserName {
		return false, "user name can only be changed once"
	}
	// check username existence in db and casdoor
	u, err := c.userStore.FindByUsername(ctx, newUserName)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return false, "failed to check new username existence in db"
		}
	}
	if u.ID > 0 {
		return false, fmt.Sprintf("new username '%s' already exists", newUserName)
	}
	if user.RegProvider != "casdoor" {
		return true, ""
	}
	casu, err := c.casc.GetUser(newUserName)
	if err != nil {
		return false, "failed to check new username existence in casdoor"
	}
	if casu != nil && casu.Id != user.UUID {
		return false, "user name already exists in casdoor"
	}
	return true, ""
}

func (c *userComponentImpl) canChangeEmail(ctx context.Context, user, opuser database.User, newEmail string) (can bool, reason string) {
	if opuser.ID != user.ID {
		return false, "email can only be changed by the user itself"
	}
	// check email existence in db and casdoor
	u, err := c.userStore.FindByEmail(ctx, newEmail)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return false, "failed to check new email existence in db"
		}
	}
	if u.ID > 0 {
		return false, fmt.Sprintf("email '%s' already exists", newEmail)
	}
	if user.RegProvider != "casdoor" {
		return true, ""
	}
	casu, err := c.casc.GetUserByEmail(newEmail)
	if err != nil {
		return false, "failed to check new email existence in casdoor"
	}
	if casu != nil && casu.Id != user.UUID {
		return false, "email already exists in casdoor"
	}

	return true, ""
}

func (c *userComponentImpl) canChangePhone(user database.User, opUser database.User, newPhone string) (bool, string) {
	if opUser.ID != user.ID {
		return false, "phone can only be changed by the user itself"
	}
	if user.RegProvider != "casdoor" {
		return true, ""
	}
	// check phone existence in casdoor
	casu, err := c.casc.GetUserByPhone(newPhone)
	if err != nil {
		return false, "failed to check new phone existence in casdoor"
	}
	if casu != nil && casu.Id != user.UUID {
		return false, "new phone already exists in casdoor"
	}
	return true, ""
}

func (c *userComponentImpl) Update(ctx context.Context, req *types.UpdateUserRequest, opUser string) error {
	c.lazyInit()

	user, err := c.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		newError := fmt.Errorf("failed to find user by name in db,error:%w", err)
		return newError
	}
	if req.Roles != nil && (opUser == "" || opUser == req.Username) {
		return fmt.Errorf("need another user to change roles of user '%s'", req.Username)
	}
	// need at least admin permission to update other user's info
	if req.Username != opUser {
		opuser, err := c.userStore.FindByUsername(ctx, opUser)
		if err != nil {
			return fmt.Errorf("failed to find op user by name in db,user: '%s', error:%w", opUser, err)
		}
		//check whether user has admin permission
		canAdmin := opuser.CanAdmin()
		if !canAdmin {
			return fmt.Errorf("failed to update user '%s', op user '%s' is not admin", req.Username, opUser)
		}
	}

	if req.Email != nil {
		err = c.upsertGitUser(user.Username, req.Nickname, user.Email, *req.Email)
		if err != nil {
			return err
		}
	}
	newUser := c.setChangedProps(&user, req)
	err = c.userStore.Update(ctx, newUser, "")
	if err != nil {
		newError := fmt.Errorf("failed to update database user '%s',error:%w", req.Username, err)
		return newError
	}

	//skip casdoor update if it's not a casdoor user
	if user.UUID == "" || user.RegProvider != "casdoor" {
		return nil
	}
	req.UUID = &user.UUID
	_, err = c.updateCasdoorUser(req)
	if err != nil {
		newError := fmt.Errorf("failed to update casdoor user '%s',error:%w", req.Username, err)
		return newError
	}

	return nil
}

// Depricated: only useful for gitea, will be removed in the future
// user registry with wechat does not have email, so git user is not created after signin
// when user set email, a git user needs to be created
func (c *userComponentImpl) upsertGitUser(username string, nickname *string, oldEmail, newEmail string) error {
	var err error
	if nickname == nil {
		nickname = &username
	}
	if oldEmail == "" {
		// create git user
		gsUserReq := gitserver.CreateUserRequest{
			Nickname: *nickname,
			Username: username,
			Email:    newEmail,
		}
		_, err = c.gs.CreateUser(gsUserReq)
		if err != nil {
			newError := fmt.Errorf("failed to create git user '%s',error:%w", username, err)
			return newError
		}
	} else {
		// update git user
		err = c.gs.UpdateUserV2(gitserver.UpdateUserRequest{
			Nickname: nickname,
			Username: username,
			Email:    &newEmail,
		})
		if err != nil {
			newError := fmt.Errorf("failed to update git user '%s',error:%w", username, err)
			return newError
		}
	}

	return nil
}

func (c *userComponentImpl) setChangedProps(oldUser *database.User, req *types.UpdateUserRequest) *database.User {
	user := *oldUser
	if req.NewUserName != nil {
		user.CanChangeUserName = false // user name can only be changed once
		user.Username = *req.NewUserName
	}
	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.Avatar != nil {
		user.Avatar = *req.Avatar
	}
	if req.Bio != nil {
		user.Bio = *req.Bio
	}
	if req.Homepage != nil {
		user.Homepage = *req.Homepage
	}
	if req.Phone != nil {
		user.Phone = *req.Phone
	}
	if req.Nickname != nil {
		user.NickName = *req.Nickname
	}
	if req.Roles != nil {
		user.SetRoles(*req.Roles)
	}

	return &user
}

func (c *userComponentImpl) Delete(ctx context.Context, operator, username string) error {
	user, err := c.userStore.FindByUsername(ctx, username)
	if err != nil {
		newError := fmt.Errorf("failed to find user by name in db,error:%w", err)
		return newError
	}
	slog.Debug("delete user from git server", slog.String("operator", operator), slog.String("username", user.Username))

	// if c.config.GitServer.Type == types.GitServerTypeGitea {
	// 	// gitea gitserver does not support delete user, you could create a pr to our repo to fix it
	// }

	if c.config.GitServer.Type == types.GitServerTypeGitaly {
		repos, err := c.repo.ByUser(ctx, user.ID)
		if err != nil {
			slog.Error("failed to find all repos for user", slog.String("username", user.Username), slog.Any("error", err))
			return fmt.Errorf("failed to find all repos for user: %v", err)
		}

		for _, repo := range repos {
			namespaceAndName := strings.Split(repo.Path, "/")
			err := c.gs.DeleteRepo(ctx, gitserver.DeleteRepoReq{
				Namespace: namespaceAndName[0],
				Name:      namespaceAndName[1],
				RepoType:  repo.RepositoryType,
			})
			if err != nil && err.Error() != GitalyRepoNotFoundErr {
				slog.Error("failed to delete user repos in git server", slog.String("username", user.Username), slog.String("repo_path", repo.Path), slog.Any("error", err))
				return fmt.Errorf("failed to delete user repos in git server: %v", err)
			}
		}
	}
	// delete user from db
	err = c.userStore.DeleteUserAndRelations(ctx, user)
	if err != nil {
		return fmt.Errorf("failed to delete user and user relations: %v", err)
	}

	// delete user from casdoor
	if user.UUID != "" {
		casUser := &casdoorsdk.User{Id: user.UUID}
		_, err = c.casc.DeleteUser(casUser)
		return fmt.Errorf("failed to delete user in casdoor: %v", err)
	}
	return err
}

// CanAdmin checks if a user has admin privileges.
//
// Parameters:
// - ctx: The context.Context object for the function.
// - username: The username of the user to check.
//
// Returns:
// - bool: True if the user has admin privileges, false otherwise.
// - error: An error if the user cannot be found in the database.
func (c *userComponentImpl) CanAdmin(ctx context.Context, username string) (bool, error) {
	user, err := c.userStore.FindByUsername(ctx, username)
	if err != nil {
		newError := fmt.Errorf("failed to find user by name '%s' in db,error:%w", username, err)
		return false, newError
	}
	return user.CanAdmin(), nil
}

// GetInternal get *full* user info by username or uuid
//
// should only be called by other *internal* services
func (c *userComponentImpl) GetInternal(ctx context.Context, userNameOrUUID string, useUUID bool) (*types.User, error) {
	var dbuser = new(database.User)
	var err error
	if useUUID {
		dbuser, err = c.userStore.FindByUUID(ctx, userNameOrUUID)
	} else {
		*dbuser, err = c.userStore.FindByUsername(ctx, userNameOrUUID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find user by name or uuid '%s' in db,error:%w", userNameOrUUID, err)
	}
	return c.buildUserInfo(ctx, dbuser, false)
}

func (c *userComponentImpl) Get(ctx context.Context, userNameOrUUID, visitorName string, useUUID bool) (*types.User, error) {
	var dbuser = new(database.User)
	var err error
	if useUUID {
		dbuser, err = c.userStore.FindByUUID(ctx, userNameOrUUID)
	} else {
		*dbuser, err = c.userStore.FindByUsername(ctx, userNameOrUUID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find user by name or uuid  '%s' in db,error:%w", userNameOrUUID, err)
	}
	userName := dbuser.Username
	var onlyBasicInfo bool
	//allow anonymous user to get basic info
	if visitorName == "" {
		onlyBasicInfo = true
	} else if userName != visitorName {
		canAdmin, err := c.CanAdmin(ctx, visitorName)
		if err != nil {
			return nil, fmt.Errorf("failed to check visitor user permission, visitor: %s, error: %w", visitorName, err)
		}

		if !canAdmin {
			onlyBasicInfo = true
		}
	}

	return c.buildUserInfo(ctx, dbuser, onlyBasicInfo)
}

func (c *userComponentImpl) CheckOperatorAndUser(ctx context.Context, operator, username string) (bool, error) {
	opUser, err := c.userStore.FindByUsername(ctx, operator)
	if err != nil {
		newError := fmt.Errorf("failed to find operator by name in db,error:%w", err)
		return true, newError
	}

	user, err := c.userStore.FindByUsername(ctx, username)
	if err != nil {
		newError := fmt.Errorf("failed to find user by name in db,error:%w", err)
		return true, newError
	}
	if !opUser.CanAdmin() {
		return false, errors.New("only admin user or the user can delete the user")
	}

	if user.CanAdmin() {
		return false, errors.New("admin user can not be deleted")
	}
	return false, nil
}

func (c *userComponentImpl) CheckIfUserHasOrgs(ctx context.Context, userName string) (bool, error) {
	var (
		err   error
		total int
	)
	if _, total, err = c.orgStore.GetUserOwnOrgs(ctx, userName); err != nil {
		return false, fmt.Errorf("failed to find orgs by username in db,error:%w", err)
	}
	return total > 0, nil
}

func (c *userComponentImpl) CheckIffUserHasRunningOrBuildingDeployments(ctx context.Context, userName string) (bool, error) {
	user, err := c.userStore.FindByUsername(ctx, userName)
	if err != nil {
		return false, fmt.Errorf("failed to find user by username in db, error: %v", err)
	}
	deploys, err := c.ds.ListAllDeployments(ctx, user.ID)
	if err != nil {
		return false, fmt.Errorf("failed to list all deployments for user %s in db, error:  %v", userName, err)
	}
	if len(deploys) > 0 {
		return true, nil
	}
	return false, nil
}

func (c *userComponentImpl) CheckIfUserHasBills(ctx context.Context, userName string) (bool, error) {
	user, err := c.userStore.FindByUsername(ctx, userName)
	if err != nil {
		return false, fmt.Errorf("failed to find user by username in db, error: %v", err)
	}
	ams, err := c.ams.ListAllByUserUUID(ctx, user.UUID)
	if err != nil {
		return false, fmt.Errorf("failed to list all account meterings for user %s in db, error: %w", userName, err)
	}
	if len(ams) > 0 {
		return true, nil
	}

	return false, nil
}

func (c *userComponentImpl) buildUserInfo(ctx context.Context, dbuser *database.User, onlyBasicInfo bool) (*types.User, error) {
	u := types.User{
		Username: dbuser.Username,
		Nickname: dbuser.NickName,
		Avatar:   dbuser.Avatar,
	}

	if !onlyBasicInfo {
		u.ID = dbuser.ID
		u.Email = dbuser.Email
		u.UUID = dbuser.UUID
		u.Bio = dbuser.Bio
		u.Homepage = dbuser.Homepage
		u.Phone = dbuser.Phone
		u.Roles = dbuser.Roles()
	}

	dborgs, err := c.orgStore.GetUserBelongOrgs(ctx, dbuser.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get orgs for user %s,error:%w", dbuser.Username, err)
	}

	if len(dborgs) > 0 {
		for _, org := range dborgs {
			u.Orgs = append(u.Orgs, types.Organization{
				Name:     org.Name,
				Nickname: org.Nickname,
				Homepage: org.Homepage,
				Logo:     org.Logo,
				OrgType:  org.OrgType,
				Verified: org.Verified,
				UserID:   org.UserID,
			})
		}
	}

	return &u, nil
}

func (c *userComponentImpl) Index(ctx context.Context, visitorName, search string, per, page int) ([]*types.User, int, error) {
	var (
		respUsers     []*types.User
		onlyBasicInfo bool
	)
	canAdmin, err := c.CanAdmin(ctx, visitorName)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to check visitor user permission, visitor: %s, error: %w", visitorName, err)
	}
	if !canAdmin {
		onlyBasicInfo = true
	}

	dbusers, count, err := c.userStore.IndexWithSearch(ctx, search, per, page)
	if err != nil {
		newError := fmt.Errorf("failed to find user by name in db,error:%w", err)
		return nil, count, newError
	}

	for _, dbuser := range dbusers {
		user := &types.User{
			Username: dbuser.Username,
			Nickname: dbuser.NickName,
			Avatar:   dbuser.Avatar,
		}

		if !onlyBasicInfo {
			user.Email = dbuser.Email
			user.UUID = dbuser.UUID
			user.Bio = dbuser.Bio
			user.Homepage = dbuser.Homepage
			user.Phone = dbuser.Phone
			user.Roles = dbuser.Roles()
		}

		respUsers = append(respUsers, user)
	}

	return respUsers, count, nil
}

func (c *userComponentImpl) Signin(ctx context.Context, code, state string) (*types.JWTClaims, string, error) {
	c.lazyInit()

	casToken, err := c.casc.GetOAuthToken(code, state)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get token from casdoor,error:%w", err)
	}
	claims, err := c.casc.ParseJwtToken(casToken.AccessToken)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse token from casdoor,error:%w", err)
	}

	cu := claims.User
	exists, err := c.userStore.IsExistByUUID(ctx, cu.Id)
	if err != nil {
		return nil, "", fmt.Errorf("failed to check user existence by name in db,error:%w", err)
	}

	var dbu *database.User
	if !exists {
		dbu, err = c.createFromCasdoorUser(ctx, cu)
		if err != nil {
			return nil, "", fmt.Errorf("failed to create user,error:%w", err)
		}
		// auto create git access token for the new user
		go func(username string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			defer cancel()
			_, err := c.tokenc.Create(ctx, &types.CreateUserTokenRequest{
				Username:    username,
				TokenName:   uuid.NewString(),
				Application: types.AccessTokenAppGit,
			})
			if err != nil {
				slog.Error("failed to create git user access token", "error", err, "username", dbu.Username)
			}
		}(dbu.Username)
	} else {
		// get user from db for username, as casdoor may have different username
		dbu, err = c.userStore.FindByUUID(ctx, cu.Id)
		if err != nil {
			return nil, "", fmt.Errorf("failed to find user by uuid in db, uuid:%s, error:%w", cu.Id, err)
		}
		// update user login time asynchronously
		go func() {
			dbu.LastLoginAt = time.Now().Format("2006-01-02 15:04:05")
			err := c.userStore.Update(ctx, dbu, "")
			if err != nil {
				slog.Error("failed to update user login time", "error", err, "username", dbu.Username)
			}
		}()
	}
	hubToken, signed, err := c.jwtc.GenerateToken(ctx, types.CreateJWTReq{
		UUID: dbu.UUID,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate jwt token,error:%w", err)
	}

	return hubToken, signed, nil
}

func (c *userComponentImpl) genUniqueName() (string, error) {
	c.lazyInit()

	if c.sfnode == nil {
		return "", fmt.Errorf("user component sfnode is nil")
	}
	id := c.sfnode.Generate().Base36()
	return "user_" + id, nil
}

func (c *userComponentImpl) updateCasdoorUser(req *types.UpdateUserRequest) (*casdoorsdk.User, error) {
	if req.UUID == nil {
		return nil, errors.New("uuid is required to update casdoor user")
	}
	//nothing to update
	if req.Email == nil && req.Phone == nil && req.NewUserName == nil && req.Nickname == nil {
		return nil, nil
	}

	c.lazyInit()

	casu, err := c.casc.GetUserByUserId(*req.UUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user from casdoor by uuid: %s,error:%w", *req.UUID, err)
	}
	if casu == nil {
		return nil, fmt.Errorf("user not found in casdoor by uuid:%s", *req.UUID)
	}
	casuCopy := *casu
	if req.Email != nil {
		casu.Email = *req.Email
	}
	if req.Phone != nil {
		casu.Phone = *req.Phone
	}
	if req.Nickname != nil {
		casu.DisplayName = *req.Nickname
	}
	// casdoor update user api don't allow empty display name, so we set it
	if casu.DisplayName == "" {
		casu.DisplayName = casu.Name
	}

	// get id by user name before changed
	id := c.casc.GetId(casu.Name)
	id = url.QueryEscape(id) // wechat user's name may contain special characters
	if req.NewUserName != nil {
		casu.Name = *req.NewUserName
	}
	_, err = c.casc.UpdateUserById(id, casu)
	return &casuCopy, err
}

func (c *userComponentImpl) lazyInit() {
	c.once.Do(func() {
		var err error
		c.casc = casdoorsdk.NewClientWithConf(c.casConfig)
		c.sfnode, err = snowflake.NewNode(1)
		if err != nil {
			slog.Error("failed to create snowflake node", "error", err)
		}
	})
}

func (c *userComponentImpl) FixUserData(ctx context.Context, userName string) error {
	err := c.gs.FixUserData(ctx, userName)
	if err != nil {
		return err
	}

	return nil
}
