//go:build !saas

package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateRepoDescriptionFromReadme_NonSaaSNoop(t *testing.T) {
	err := UpdateRepoDescriptionFromReadme(context.TODO(), UpdateRepoDescriptionFromReadmeReq{})

	require.NoError(t, err)
}
