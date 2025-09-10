package scenariomgr

import (
	"fmt"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
)

// DataProvider is a component that provides data for the scenario manager,
// it's used to access database to get data for the scenario manager.
type DataProvider struct {
	notificationStorage database.NotificationStore
	userSvcClient       rpc.UserSvcClient
}

func NewDataProvider(storage database.NotificationStore, config *config.Config) *DataProvider {
	userSvcAddr := fmt.Sprintf("%s:%d", config.User.Host, config.User.Port)
	userRpcClient := rpc.NewUserSvcHttpClient(userSvcAddr, rpc.AuthWithApiKey(config.APIToken))
	return &DataProvider{
		notificationStorage: storage,
		userSvcClient:       userRpcClient,
	}
}

func (d *DataProvider) GetNotificationStorage() database.NotificationStore {
	return d.notificationStorage
}

func (d *DataProvider) GetUserSvcClient() rpc.UserSvcClient {
	return d.userSvcClient
}
