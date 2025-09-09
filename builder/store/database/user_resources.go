package database

import (
	"context"
	"fmt"
	"opencsg.com/csghub-server/common/errorx"
	"time"
)

type userResourcesStoreImpl struct {
	db *DB
}

type UserResourcesStore interface {
	// add user resources
	AddUserResources(ctx context.Context, userResources *UserResources) error
	// get user resources by user uid
	GetUserResourcesByUserUID(ctx context.Context, per, page int, userId string) (userResources []UserResources, total int, err error)
	// get need reserved user resources which is not deployed and not expired
	GetReservedUserResources(ctx context.Context, userId string, clusterId string) ([]UserResources, error)
	// update deploy id
	UpdateDeployId(ctx context.Context, userResources *UserResources) error
	// find user resources by order detail id
	FindUserResourcesByOrderDetailId(ctx context.Context, userUId string, orderDetailId int64) (*UserResources, error)
	// delete user resources by order detail id
	DeleteUserResourcesByOrderDetailId(ctx context.Context, userUid string, orderDetailId int64) error
}

func NewUserResourcesStore() UserResourcesStore {
	return &userResourcesStoreImpl{
		db: defaultDB,
	}
}

func NewUserResourcesStoreWithDB(db *DB) UserResourcesStore {
	return &userResourcesStoreImpl{
		db: db,
	}
}

type UserResources struct {
	ID            int64          `bun:",pk,autoincrement" json:"id"`
	UserUID       string         `bun:",notnull" json:"user_uid"`
	OrderId       string         `bun:",notnull" json:"order_id"`
	OrderDetailId int64          `bun:",notnull,unique" json:"order_detail_id"`
	ResourceId    int64          `bun:",notnull" json:"resource_id"`
	DeployId      int64          `bun:",notnull" json:"deploy_id"`
	XPUNum        int            `bun:",notnull" json:"xpu_num"`
	PayMode       string         `bun:",notnull" json:"pay_mode"`
	Price         float64        `bun:",notnull" json:"price"`
	CreatedAt     time.Time      `bun:",nullzero,notnull,skipupdate,default:current_timestamp" json:"created_at"`
	StartTime     time.Time      `bun:",nullzero,notnull,skipupdate,default:current_timestamp" json:"start_time"`
	EndTime       time.Time      `bun:",nullzero,notnull,skipupdate" json:"end_time"`
	SpaceResource *SpaceResource `bun:"rel:belongs-to,join:resource_id=id" json:"resource"`
	Deploy        *Deploy        `bun:"rel:belongs-to,join:deploy_id=id" json:"deploy"`
}

// add user resources
func (s *userResourcesStoreImpl) AddUserResources(ctx context.Context, userResources *UserResources) error {
	res, err := s.db.Core.NewInsert().Model(userResources).Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("failed to create user resource in db ,error:%w", err)
	}
	return nil
}

// get user resources by user uid
func (s *userResourcesStoreImpl) GetUserResourcesByUserUID(ctx context.Context, per, page int, userId string) (userResources []UserResources, total int, err error) {

	query := s.db.Operator.Core.
		NewSelect().
		Model(&userResources).
		Relation("SpaceResource").
		Relation("Deploy").
		Where("user_resources.user_uid = ?", userId)

	query = query.Order("user_resources.created_at DESC").
		Limit(per).
		Offset((page - 1) * per)

	err = query.Scan(ctx)
	if err != nil {
		return userResources, 0, errorx.HandleDBError(err, nil)
	}
	total, err = query.Count(ctx)
	if err != nil {
		return userResources, total, errorx.HandleDBError(err, nil)
	}
	return
}

// get need reserved user resources which is not deployed and not expired
func (s *userResourcesStoreImpl) GetReservedUserResources(ctx context.Context, userId string, clusterId string) ([]UserResources, error) {
	var userResources []UserResources
	query := s.db.Operator.Core.
		NewSelect().
		Model(&userResources).
		Relation("SpaceResource").
		Where("user_resources.deploy_id = ?", 0).
		Where("user_resources.end_time > ?", time.Now())
	if userId != "" {
		query.Where("user_resources.user_uid = ?", userId)
	}
	err := query.Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	if clusterId == "" {
		return userResources, nil
	}
	var filteredUserResources []UserResources
	for _, userResource := range userResources {
		if userResource.SpaceResource.ClusterID == clusterId {
			filteredUserResources = append(filteredUserResources, userResource)
		}
	}
	return filteredUserResources, nil
}

// update deploy id
func (s *userResourcesStoreImpl) UpdateDeployId(ctx context.Context, userResources *UserResources) error {
	res, err := s.db.Core.NewUpdate().Model(userResources).WherePK().Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("failed to update user resource in db ,error:%w", err)
	}
	return nil
}

// find user resources by order detail id
func (s *userResourcesStoreImpl) FindUserResourcesByOrderDetailId(ctx context.Context, userUId string, orderDetailId int64) (*UserResources, error) {
	var userResources UserResources
	query := s.db.Operator.Core.
		NewSelect().
		Model(&userResources).
		Where("user_resources.order_detail_id = ?", orderDetailId)
	if userUId != "" {
		query.Where("user_resources.user_uid = ?", userUId)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}
	return &userResources, nil
}

// delete user resources by order detail id
func (s *userResourcesStoreImpl) DeleteUserResourcesByOrderDetailId(ctx context.Context, userUid string, orderDetailId int64) error {
	res, err := s.db.Core.NewDelete().Model(&UserResources{}).
		Where("order_detail_id = ?", orderDetailId).
		Where("user_uid = ?", userUid).
		Exec(ctx)
	if err := assertAffectedOneRow(res, err); err != nil {
		return fmt.Errorf("failed to delete user resource in db ,error:%w", err)
	}
	return nil
}
