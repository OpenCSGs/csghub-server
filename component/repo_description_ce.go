//go:build !saas

package component

import "context"

func UpdateRepoDescriptionFromReadme(ctx context.Context, req UpdateRepoDescriptionFromReadmeReq) error {
	return nil
}
