package component

import (
	"context"
	"fmt"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/workhub"
)

func newRepositoryDeletionJobClient() (database.RepositoryDeletionJobClient, error) {
	return newRepositoryDeletionJobClientWithDB(database.GetDB())
}

func newRepositoryDeletionJobClientWithDB(db *database.DB) (database.RepositoryDeletionJobClient, error) {
	if db == nil || db.BunDB == nil {
		return nil, nil
	}
	client, err := workhub.NewJobClient(context.Background(), db.BunDB)
	if err != nil {
		return nil, fmt.Errorf("create repository deletion job client: %w", err)
	}
	return workhub.NewRepositoryDeletionJobClient(client), nil
}
