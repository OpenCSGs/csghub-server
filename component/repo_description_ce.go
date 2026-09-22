//go:build !saas

package component

import "context"

func UpdateRepoDescriptionFromReadme(ctx context.Context, req UpdateRepoDescriptionFromReadmeReq) error {
	return nil
}

func UpdateRepoDescriptionFromReadmeWithResult(ctx context.Context, req UpdateRepoDescriptionFromReadmeReq) (RepoDescriptionUpdateStatus, error) {
	return RepoDescriptionSkipped, nil
}
