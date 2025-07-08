package scenariomgr

import (
	"opencsg.com/csghub-server/builder/store/database"
)

// DataProvider is a component that provides data for the scenario manager,
// it's used to access database to get data for the scenario manager.
type DataProvider struct {
	notificationStorage database.NotificationStore
}

func NewDataProvider(storage database.NotificationStore) *DataProvider {
	return &DataProvider{
		notificationStorage: storage,
	}
}

func (d *DataProvider) GetNotificationStorage() database.NotificationStore {
	return d.notificationStorage
}
