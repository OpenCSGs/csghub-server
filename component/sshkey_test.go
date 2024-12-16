package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

const testKey = `
ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCn4yeHw9InFrZIxYxFhs5Giam76NPIJ1kOqEq1xvWz4vJJMGkoqosTsqUf+V4Pj18qSUbSEDbwibzkIAPFNRiF1lQWgpFvZrZsTmD6rV1ODYjGPu5HLHqjCY/ffY+n/cAz66sZ5TQUMh+9HmUkVriu/Flfo7dWrbsrC73vgfVptSzSIEehkm4wL40XaZI4wQ7JffdXyqz5CU/lK+CFaPU2nLnxVoL9CEaFbCglcP4sO2jir2Rcx5ZNBMHYpsqk9N4cOxpS/IA9YX2tla3o4wltJoO83Vp0qH1ds15WBAlwUAdpJGDajh93kgYki6Kn2v41IgmqgFcXpmBQ+48QZXfh
`

func TestSSHKeyComponent_Create(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSSHKeyComponent(ctx, t)

	req := &types.CreateSSHKeyRequest{
		Username: "user",
		Name:     "n",
		Content:  testKey,
	}
	sc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{ID: 1}, nil)
	sc.mocks.stores.SSHMock().EXPECT().FindByNameAndUserID(ctx, "n", int64(1)).Return(
		&database.SSHKey{}, nil,
	)
	sc.mocks.stores.SSHMock().EXPECT().FindByKeyContent(ctx, testKey).Return(&database.SSHKey{}, nil)
	sc.mocks.gitServer.EXPECT().CreateSSHKey(req).Return(&database.SSHKey{}, nil)
	sc.mocks.stores.SSHMock().EXPECT().Create(ctx, &database.SSHKey{
		UserID:            1,
		FingerprintSHA256: "DZMgXySN8FuYZo2qvIAZOXNB0J81NMAv1SikyHvCPmw",
	}).Return(&database.SSHKey{}, nil)

	data, err := sc.Create(ctx, req)
	require.NoError(t, err)
	require.Equal(t, &database.SSHKey{}, data)

}

func TestSSHKeyComponent_Index(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSSHKeyComponent(ctx, t)

	sc.mocks.stores.SSHMock().EXPECT().Index(ctx, "user", 10, 1).Return(
		[]database.SSHKey{{Name: "a"}}, nil,
	)

	data, err := sc.Index(ctx, "user", 10, 1)
	require.Nil(t, err)
	require.Equal(t, data, []database.SSHKey{{Name: "a"}})
}

func TestSSHKeyComponent_Delete(t *testing.T) {
	ctx := context.TODO()
	sc := initializeTestSSHKeyComponent(ctx, t)

	sc.mocks.stores.SSHMock().EXPECT().FindByUsernameAndName(ctx, "user", "key").Return(
		database.SSHKey{ID: 1, GitID: 123}, nil,
	)
	sc.mocks.gitServer.EXPECT().DeleteSSHKey(123).Return(nil)
	sc.mocks.stores.SSHMock().EXPECT().Delete(ctx, int64(1)).Return(nil)

	err := sc.Delete(ctx, "user", "key")
	require.Nil(t, err)
}
