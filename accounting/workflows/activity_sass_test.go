//go:build saas

package workflows

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mockcomponent "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/accounting/component"
)

func TestSubscriptionActivity_ScanAndConfirmPeriodicalSubscriptions(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockSubComp := mockcomponent.NewMockSubscriptionComponent(t)
		act := &subscriptionActivityImpl{
			subComp: mockSubComp,
		}

		mockSubComp.EXPECT().ScanAndConfirmSubscriptions(ctx).Return(nil)

		err := act.ScanAndConfirmPeriodicalSubscriptions(ctx)
		require.Nil(t, err)
	})

	t.Run("error", func(t *testing.T) {
		mockSubComp := mockcomponent.NewMockSubscriptionComponent(t)
		act := &subscriptionActivityImpl{
			subComp: mockSubComp,
		}

		mockSubComp.EXPECT().ScanAndConfirmSubscriptions(ctx).Return(assert.AnError)

		err := act.ScanAndConfirmPeriodicalSubscriptions(ctx)
		require.Error(t, err)
	})
}

func TestExpiredPresentActivity_ProcessExpiredPresents(t *testing.T) {
	ctx := context.Background()
	activityID := int64(1001)

	t.Run("success", func(t *testing.T) {
		mockPresentComp := mockcomponent.NewMockAccountingPresentComponent(t)
		act := &expiredPresentActivityImpl{
			presentComp: mockPresentComp,
		}

		mockPresentComp.EXPECT().ProcessExpiredPresents(ctx, activityID).Return(nil)

		err := act.ProcessExpiredPresents(ctx, activityID)
		require.Nil(t, err)
	})

	t.Run("error", func(t *testing.T) {
		mockPresentComp := mockcomponent.NewMockAccountingPresentComponent(t)
		act := &expiredPresentActivityImpl{
			presentComp: mockPresentComp,
		}

		mockPresentComp.EXPECT().ProcessExpiredPresents(ctx, activityID).Return(assert.AnError)

		err := act.ProcessExpiredPresents(ctx, activityID)
		require.Error(t, err)
	})
}
