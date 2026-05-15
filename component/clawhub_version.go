package component

import (
	"context"
	"fmt"
	"time"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/errorx"
)

// createOrUpdateVersion creates a new version or updates existing one
// For "latest" version, it always updates the existing record if found
func (c *clawHubComponent) createOrUpdateVersion(ctx context.Context, skillID int64, version string, commitHash string, changelog string) (*database.SkillVersion, error) {
	// Check if version already exists
	existingVersion, err := c.skillVersionStore.BySkillIDAndVersion(ctx, skillID, version)
	if err == nil && existingVersion != nil {
		// Version exists, update it (for "latest" or any existing version)
		existingVersion.UpdatedAt = time.Now()
		if commitHash != "" {
			existingVersion.Hash = commitHash
		}
		if changelog != "" {
			existingVersion.Changelog = changelog
		}
		err = c.skillVersionStore.Update(ctx, *existingVersion)
		if err != nil {
			return nil, errorx.SkillVersionUpdateFailed(err, errorx.Ctx().Set("skill_id", fmt.Sprintf("%d", skillID)).Set("version", version))
		}
		return existingVersion, nil
	}

	// Version doesn't exist, create new one
	sv := database.SkillVersion{
		SkillID:   skillID,
		Version:   version,
		Hash:      commitHash,
		Changelog: changelog,
	}
	newVersion, err := c.skillVersionStore.Create(ctx, sv)
	if err != nil {
		return nil, errorx.SkillVersionCreateFailed(err, errorx.Ctx().Set("skill_id", fmt.Sprintf("%d", skillID)).Set("version", version))
	}
	return newVersion, nil
}
