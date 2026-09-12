package database

import (
	"context"
	"time"

	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

// RepositoryAuthorization stores a direct repository grant for a user or organization.
type RepositoryAuthorization struct {
	ID           int64                     `bun:",pk,autoincrement" json:"id"`
	RepositoryID int64                     `bun:",notnull" json:"repository_id"`
	SubjectType  types.RepoAuthSubjectType `bun:",notnull" json:"subject_type"`
	SubjectID    int64                     `bun:",notnull" json:"subject_id"`
	SubjectUUID  string                    `bun:",notnull" json:"subject_uuid"`
	Role         types.UserRole            `bun:",notnull" json:"role"`
	CreateUserID int64                     `bun:",notnull" json:"create_user_id"`
	CreatedAt    time.Time                 `bun:",nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt    time.Time                 `bun:",nullzero,notnull,default:current_timestamp" json:"updated_at"`
}

// RepositoryAuthorizationStore persists direct repository grants.
type RepositoryAuthorizationStore interface {
	// Create adds a direct repository grant or updates the existing grant for the same subject.
	Create(ctx context.Context, authorization *RepositoryAuthorization) error
	// Find retrieves a direct repository grant by repository and subject.
	Find(ctx context.Context, repositoryID, subjectID int64, subjectType types.RepoAuthSubjectType) (*RepositoryAuthorization, error)
	// List returns direct grants for a repository with pagination and the total count.
	List(ctx context.Context, repositoryID int64, per, page int) ([]RepositoryAuthorization, int, error)
	// ListByRepository returns all direct grants for a repository, including grants whose subjects were soft-deleted.
	ListByRepository(ctx context.Context, repositoryID int64) ([]RepositoryAuthorization, error)
	// UpdateRole changes the role of an existing direct repository grant.
	UpdateRole(ctx context.Context, repositoryID, subjectID int64, subjectType types.RepoAuthSubjectType, role types.UserRole) error
	// Delete removes a direct repository grant.
	Delete(ctx context.Context, repositoryID, subjectID int64, subjectType types.RepoAuthSubjectType) error
	// DeleteBySubject removes all direct repository grants for a subject.
	DeleteBySubject(ctx context.Context, subjectType types.RepoAuthSubjectType, subjectID int64) error
	// DeleteByRepository removes all direct grants for a repository.
	DeleteByRepository(ctx context.Context, repositoryID int64) error
	// ListBySubject returns all direct repository grants for a subject.
	ListBySubject(ctx context.Context, subjectType types.RepoAuthSubjectType, subjectID int64) ([]RepositoryAuthorization, error)
}

type repositoryAuthorizationStoreImpl struct{ db *DB }

// NewRepositoryAuthorizationStore creates a direct repository authorization store.
func NewRepositoryAuthorizationStore() RepositoryAuthorizationStore {
	return NewRepositoryAuthorizationStoreWithDB(defaultDB)
}

// NewRepositoryAuthorizationStoreWithDB creates a direct repository authorization store with the provided database.
func NewRepositoryAuthorizationStoreWithDB(db *DB) RepositoryAuthorizationStore {
	return &repositoryAuthorizationStoreImpl{db: db}
}

// roleValid reports whether a repository authorization role is supported.
func (s *repositoryAuthorizationStoreImpl) roleValid(role types.UserRole) bool {
	return role == types.UserRead || role == types.UserWrite
}

// subjectTypeValid reports whether a repository authorization subject type is supported.
func subjectTypeValid(subjectType types.RepoAuthSubjectType) bool {
	return subjectType.IsValid()
}

// Create adds a direct repository grant or updates the existing grant for the same subject.
func (s *repositoryAuthorizationStoreImpl) Create(ctx context.Context, authorization *RepositoryAuthorization) error {
	if !subjectTypeValid(authorization.SubjectType) || !s.roleValid(authorization.Role) {
		return errorx.ErrReqParamInvalid
	}
	_, err := s.db.Operator.Core.NewInsert().Model(authorization).
		On("CONFLICT (repository_id, subject_type, subject_id) DO UPDATE").
		Set("subject_uuid = EXCLUDED.subject_uuid").
		Set("role = EXCLUDED.role").
		Set("updated_at = CURRENT_TIMESTAMP").Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

// Find retrieves a direct repository grant by repository and subject.
func (s *repositoryAuthorizationStoreImpl) Find(ctx context.Context, repositoryID, subjectID int64, subjectType types.RepoAuthSubjectType) (*RepositoryAuthorization, error) {
	if !subjectTypeValid(subjectType) {
		return nil, errorx.ErrReqParamInvalid
	}
	result := new(RepositoryAuthorization)
	err := s.db.Operator.Core.NewSelect().Model(result).
		Where("repository_id = ? AND subject_id = ? AND subject_type = ?", repositoryID, subjectID, subjectType).
		Limit(1).Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, errorx.Ctx().Set("repository_id", repositoryID))
	}
	return result, nil
}

// List returns active direct grants for a repository with pagination and the total count.
func (s *repositoryAuthorizationStoreImpl) List(ctx context.Context, repositoryID int64, per, page int) ([]RepositoryAuthorization, int, error) {
	if per <= 0 {
		per = 50
	}
	if page <= 0 {
		page = 1
	}
	items := make([]RepositoryAuthorization, 0)
	// Filter stale grants in SQL so pagination and total count only include active subjects.
	query := s.db.Operator.Core.NewSelect().Model(&items).
		Where(`repository_id = ? AND (
			(subject_type = ? AND EXISTS (
				SELECT 1 FROM users WHERE users.id = subject_id AND users.deleted_at IS NULL
			)) OR
			(subject_type = ? AND EXISTS (
				SELECT 1 FROM organizations WHERE organizations.id = subject_id AND organizations.deleted_at IS NULL
			))
		)`, repositoryID, types.RepoAuthSubjectUser, types.RepoAuthSubjectOrganization)
	count, err := query.Count(ctx)
	if err != nil {
		return nil, 0, errorx.HandleDBError(err, nil)
	}
	err = query.Order("id DESC").Limit(per).Offset((page - 1) * per).Scan(ctx)
	return items, count, errorx.HandleDBError(err, nil)
}

// ListByRepository returns all direct grants for repository cleanup.
func (s *repositoryAuthorizationStoreImpl) ListByRepository(ctx context.Context, repositoryID int64) (items []RepositoryAuthorization, err error) {
	items = make([]RepositoryAuthorization, 0)
	err = s.db.Operator.Core.NewSelect().Model(&items).
		Where("repository_id = ?", repositoryID).
		Order("id DESC").Scan(ctx)
	return items, errorx.HandleDBError(err, nil)
}

// UpdateRole changes the role of an existing direct repository grant.
func (s *repositoryAuthorizationStoreImpl) UpdateRole(ctx context.Context, repositoryID, subjectID int64, subjectType types.RepoAuthSubjectType, role types.UserRole) error {
	if !subjectTypeValid(subjectType) || !s.roleValid(role) {
		return errorx.ErrReqParamInvalid
	}
	result, err := s.db.Operator.Core.NewUpdate().Model((*RepositoryAuthorization)(nil)).
		Set("role = ?", role).Set("updated_at = CURRENT_TIMESTAMP").
		Where("repository_id = ? AND subject_id = ? AND subject_type = ?", repositoryID, subjectID, subjectType).Exec(ctx)
	if err != nil {
		return errorx.HandleDBError(err, nil)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return errorx.ErrRepoAuthorizationNotFound
	}
	return nil
}

// Delete removes a direct repository grant for a repository subject.
func (s *repositoryAuthorizationStoreImpl) Delete(ctx context.Context, repositoryID, subjectID int64, subjectType types.RepoAuthSubjectType) error {
	if !subjectTypeValid(subjectType) {
		return errorx.ErrReqParamInvalid
	}
	_, err := s.db.Operator.Core.NewDelete().Model((*RepositoryAuthorization)(nil)).
		Where("repository_id = ? AND subject_id = ? AND subject_type = ?", repositoryID, subjectID, subjectType).Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

// DeleteBySubject removes all direct repository grants for a subject.
func (s *repositoryAuthorizationStoreImpl) DeleteBySubject(ctx context.Context, subjectType types.RepoAuthSubjectType, subjectID int64) error {
	if !subjectTypeValid(subjectType) {
		return errorx.ErrReqParamInvalid
	}
	_, err := s.db.Operator.Core.NewDelete().Model((*RepositoryAuthorization)(nil)).
		Where("subject_type = ? AND subject_id = ?", subjectType, subjectID).Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

// DeleteByRepository removes all direct grants for a repository.
func (s *repositoryAuthorizationStoreImpl) DeleteByRepository(ctx context.Context, repositoryID int64) error {
	_, err := s.db.Operator.Core.NewDelete().Model((*RepositoryAuthorization)(nil)).
		Where("repository_id = ?", repositoryID).Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

// ListBySubject returns all direct repository grants for a subject.
func (s *repositoryAuthorizationStoreImpl) ListBySubject(ctx context.Context, subjectType types.RepoAuthSubjectType, subjectID int64) ([]RepositoryAuthorization, error) {
	if !subjectTypeValid(subjectType) {
		return nil, errorx.ErrReqParamInvalid
	}
	items := make([]RepositoryAuthorization, 0)
	err := s.db.Operator.Core.NewSelect().Model(&items).
		Where("subject_type = ? AND subject_id = ?", subjectType, subjectID).
		Order("repository_id ASC").Scan(ctx)
	return items, errorx.HandleDBError(err, nil)
}

var _ RepositoryAuthorizationStore = (*repositoryAuthorizationStoreImpl)(nil)
