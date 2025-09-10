//go:build saas

package deploy

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestDeployerConsumer_stopUserRunningDeploy(t *testing.T) {
	tester := newTestDeployer(t)

	notify := types.AcctNotify{
		Uuid:       uuid.New(),
		UserUUID:   "test-user-uuid",
		CreatedAt:  time.Now(),
		ReasonCode: types.ACCTStopDeploy,
		ReasonMsg:  fmt.Sprintf("user balance %f is less than threshold %d and stop deploy", -10.0, 5000),
	}

	buf, err := json.Marshal(notify)
	require.Nil(t, err)

	tester.mocks.stores.UserMock().EXPECT().FindByUUID(mock.Anything, notify.UserUUID).Return(&database.User{
		ID:   1,
		UUID: notify.UserUUID,
	}, nil)

	tester.mocks.stores.DeployTaskMock().EXPECT().GetRunningDeployByUserID(mock.Anything, int64(1)).Return([]database.Deploy{
		{
			ID:            1,
			UserID:        1,
			RepoID:        1,
			GitPath:       "ns/reponame",
			SpaceID:       1,
			OrderDetailID: 0,
			SvcName:       "test-svc-name",
			ClusterID:     "cluster-id-1",
			SKU:           "1",
		},
	}, nil)

	tester.mocks.runner.EXPECT().Stop(mock.Anything, &types.StopRequest{
		ID:        int64(1),
		OrgName:   "ns",
		RepoName:  "reponame",
		SvcName:   "test-svc-name",
		ClusterID: "cluster-id-1",
	}).Return(&types.StopResponse{}, nil)

	tester.mocks.runner.EXPECT().Exist(mock.Anything, &types.CheckRequest{
		ID:        int64(1),
		OrgName:   "ns",
		RepoName:  "reponame",
		SvcName:   "test-svc-name",
		ClusterID: "cluster-id-1",
	}).Return(&types.StatusResponse{
		Code: common.Stopped,
	}, nil)

	tester.mocks.stores.DeployTaskMock().EXPECT().StopDeploy(mock.Anything, types.SpaceRepo, int64(1), int64(1), int64(1)).Return(nil)

	tester.mocks.acctClent.EXPECT().QueryPricesBySKUType("", types.AcctPriceListReq{
		SkuType:    types.SKUCSGHub,
		SkuKind:    strconv.Itoa(int(types.SKUPayAsYouGo)),
		ResourceID: "1",
		Per:        1,
		Page:       1,
	}).Return(&database.PriceResp{
		Prices: []database.AccountPrice{
			{
				ID:         1,
				SkuType:    types.SKUCSGHub,
				SkuKind:    types.SKUPayAsYouGo,
				ResourceID: "1",
				SkuPrice:   100.0,
			},
		},
		Total: 0,
	}, nil)

	err = tester.stopUserRunningDeploys(buf)
	require.Nil(t, err)
}
