package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/bwmarrin/snowflake"
	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type UserComponent struct {
	us   *database.UserStore
	os   *database.OrgStore
	ns   *database.NamespaceStore
	gs   gitserver.GitServer
	jwtc *JwtComponent

	casc      *casdoorsdk.Client
	casConfig *casdoorsdk.AuthConfig
	once      *sync.Once
	sfnode    *snowflake.Node
}

func NewUserComponent(config *config.Config) (*UserComponent, error) {
	var err error
	c := &UserComponent{}
	c.us = database.NewUserStore()
	c.os = database.NewOrgStore()
	c.ns = database.NewNamespaceStore()
	c.jwtc = NewJwtComponent(config.JWT.SigningKey, config.JWT.ValidHour)
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

	return c, nil
}

// This function creates a user when user register from portal, without casdoor
func (c *UserComponent) createFromPortalRegistry(ctx context.Context, req types.CreateUserRequest) (*database.User, error) {
	// Panic if the function has not been implemented
	panic("implement me later")
}

func (c *UserComponent) createFromCasdoorUser(ctx context.Context, cu casdoorsdk.User) (*database.User, error) {
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
		user.Email = gsUserResp.Email
		user.Password = gsUserResp.Password
	}
	err = c.us.Create(ctx, user, namespace)
	if err != nil {
		newError := fmt.Errorf("failed to create user in db,error:%w", err)
		return nil, newError
	}

	return user, nil
}

func (c *UserComponent) ChangeUserName(ctx context.Context, oldUserName, newUserName, opUser string) error {
	if oldUserName != opUser {
		return fmt.Errorf("user name can only be changed by user self, user: '%s', op user: '%s'", oldUserName, opUser)
	}

	user, err := c.us.FindByUsername(ctx, oldUserName)
	if err != nil {
		return fmt.Errorf("failed to find user by old name in db,error:%w", err)
	}

	if !user.CanChangeUserName {
		return fmt.Errorf("user name can not be changed")
	}

	newUser, err := c.us.FindByUsername(ctx, newUserName)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to find user by new name in db,error:%w", err)
	}
	if newUser.ID > 0 {
		return fmt.Errorf("user name '%s' already exists", newUserName)
	}

	err = c.us.ChangeUserName(ctx, oldUserName, newUserName)
	if err != nil {
		return fmt.Errorf("failed to change user name in db,error:%w", err)
	}

	//skip casdoor update if it's not a casdoor user
	if user.UUID == "" || user.RegProvider != "casdoor" {
		return nil
	}

	c.lazyInit()

	err = c.updateCasdoorUser(&types.UpdateUserRequest{
		UUID:        &user.UUID,
		NewUserName: &newUserName,
	})
	if err != nil {
		newError := fmt.Errorf("failed to update casdoor user, uuid:'%s',error:%w", user.UUID, err)
		return newError
	}
	return nil
}

func (c *UserComponent) Update(ctx context.Context, req *types.UpdateUserRequest, opUser string) error {
	c.lazyInit()

	user, err := c.us.FindByUsername(ctx, req.Username)
	if err != nil {
		newError := fmt.Errorf("failed to find user by name in db,error:%w", err)
		return newError
	}
	if req.Roles != nil && (opUser == "" || opUser == req.Username) {
		return fmt.Errorf("need another user to change roles of user '%s'", req.Username)
	}
	// need at least admin permission to update other user's info
	if req.Username != opUser {
		opuser, err := c.us.FindByUsername(ctx, opUser)
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

	c.setChangedProps(&user, req)
	err = c.us.Update(ctx, &user)
	if err != nil {
		newError := fmt.Errorf("failed to update database user '%s',error:%w", req.Username, err)
		return newError
	}

	//skip casdoor update if it's not a casdoor user
	if req.UUID == nil || user.RegProvider != "casdoor" {
		return nil
	}
	err = c.updateCasdoorUser(req)
	if err != nil {
		newError := fmt.Errorf("failed to update casdoor user '%s',error:%w", req.Username, err)
		return newError
	}

	return nil
}

// user registery with wechat does not have email, so git user is not created after signin
// when user set email, a git user needs to be created
func (c *UserComponent) upsertGitUser(username string, nickname *string, oldEmail, newEmail string) error {
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

func (c *UserComponent) setChangedProps(user *database.User, req *types.UpdateUserRequest) {
	if req.Email != nil {
		user.Email = *req.Email
		if user.CanChangeUserName {
			user.CanChangeUserName = false
			slog.Info("use set email, disallow to change user name later (can_change_user_name=false)",
				slog.String("username", user.Username), slog.String("email", user.Email))
		}
	}
	if req.UUID != nil {
		user.UUID = *req.UUID
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
}

func (c *UserComponent) Delete(ctx context.Context, username string) error {
	user, err := c.us.FindByUsername(ctx, username)
	if err != nil {
		newError := fmt.Errorf("failed to find user by name in db,error:%w", err)
		return newError
	}
	// TODO:delete user from git server
	slog.Debug("delete user from git server", slog.String("username", user.Username))

	// TODO:delete user from db
	// err = c.us.Delete(ctx, user)

	// delete user from casdoor
	casUser := &casdoorsdk.User{}
	_, err = c.casc.DeleteUser(casUser)
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
func (c *UserComponent) CanAdmin(ctx context.Context, username string) (bool, error) {
	user, err := c.us.FindByUsername(ctx, username)
	if err != nil {
		newError := fmt.Errorf("failed to find user by name '%s' in db,error:%w", username, err)
		return false, newError
	}
	return user.CanAdmin(), nil
}

func (c *UserComponent) Get(ctx context.Context, userName, visitorName string) (*types.User, error) {
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

	dbuser, err := c.us.FindByUsername(ctx, userName)
	if err != nil {
		newError := fmt.Errorf("failed to find user by name in db,error:%w", err)
		return nil, newError
	}

	u := types.User{
		Username: dbuser.Username,
		Nickname: dbuser.NickName,
		Avatar:   dbuser.Avatar,
	}

	if !onlyBasicInfo {
		u.Email = dbuser.Email
		u.UUID = dbuser.UUID
		u.Bio = dbuser.Bio
		u.Homepage = dbuser.Homepage
		u.Phone = dbuser.Phone
		u.Roles = dbuser.Roles()
	}

	dborgs, err := c.os.GetUserBelongOrgs(ctx, dbuser.ID)
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
			})
		}
	}

	return &u, nil
}

func (c *UserComponent) Signin(ctx context.Context, code, state string) (*types.JWTClaims, string, error) {
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
	exists, err := c.us.IsExistByUUID(ctx, cu.Id)
	if err != nil {
		return nil, "", fmt.Errorf("failed to check user existance by name in db,error:%w", err)
	}

	if !exists {
		_, err = c.createFromCasdoorUser(ctx, cu)
		if err != nil {
			return nil, "", fmt.Errorf("failed to create user,error:%w", err)
		}
	}
	// get user from db for username, as casdoor may have different username
	dbu, err := c.us.FindByUUID(ctx, cu.Id)
	if err != nil {
		return nil, "", fmt.Errorf("failed to find user by uuid in db, uuid:%s, error:%w", cu.Id, err)
	}
	hubToken, signed, err := c.jwtc.GenerateToken(ctx, types.CreateJWTReq{
		UUID:        dbu.UUID,
		CurrentUser: dbu.Username,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate jwt token,error:%w", err)
	}

	return hubToken, signed, nil
}

func (c *UserComponent) genUniqueName() (string, error) {
	c.lazyInit()

	if c.sfnode == nil {
		return "", fmt.Errorf("user component sfnode is nil")
	}
	id := c.sfnode.Generate().Base36()
	return "user_" + id, nil
}

func (c *UserComponent) updateCasdoorUser(req *types.UpdateUserRequest) error {
	c.lazyInit()

	casu, err := c.casc.GetUserByUserId(*req.UUID)
	if err != nil {
		return fmt.Errorf("failed to get user from casdoor,error:%w", err)
	}
	if casu == nil {
		return fmt.Errorf("user not exists in casdoor")
	}
	var cols []string
	if req.Email != nil {
		casu.Email = *req.Email
		cols = append(cols, "email")
	}
	if req.Phone != nil {
		casu.Phone = *req.Phone
		cols = append(cols, "phone")
	}

	if len(cols) == 0 {
		return nil
	}

	// casdoor update user api don't allow empty display name, so we set it but not update it
	if casu.DisplayName == "" {
		casu.DisplayName = casu.Name
	}

	_, err = c.casc.UpdateUserForColumns(casu, cols)
	return err
}

func (c *UserComponent) lazyInit() {
	c.once.Do(func() {
		var err error
		c.casc = casdoorsdk.NewClientWithConf(c.casConfig)
		c.sfnode, err = snowflake.NewNode(1)
		if err != nil {
			slog.Error("failed to create snowflake node", "error", err)
		}
	})
}

func (c *UserComponent) FixUserData(ctx context.Context, userName string) error {
	err := c.gs.FixUserData(ctx, userName)
	if err != nil {
		return err
	}

	return nil
}
