package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type userVisibleAPIKeyRepoStub struct {
	validIDs []int64
	key      *APIKey
}

func (s *userVisibleAPIKeyRepoStub) Create(context.Context, *APIKey) error { panic("unexpected") }
func (s *userVisibleAPIKeyRepoStub) GetByID(context.Context, int64) (*APIKey, error) {
	if s.key == nil {
		return nil, ErrAPIKeyNotFound
	}
	clone := *s.key
	return &clone, nil
}
func (s *userVisibleAPIKeyRepoStub) GetKeyAndOwnerID(context.Context, int64) (string, int64, error) {
	panic("unexpected")
}
func (s *userVisibleAPIKeyRepoStub) GetByKey(context.Context, string) (*APIKey, error) {
	panic("unexpected")
}
func (s *userVisibleAPIKeyRepoStub) GetByKeyForAuth(context.Context, string) (*APIKey, error) {
	panic("unexpected")
}
func (s *userVisibleAPIKeyRepoStub) GetBySourceForUserGroup(context.Context, int64, *int64, string) (*APIKey, error) {
	panic("unexpected")
}
func (s *userVisibleAPIKeyRepoStub) Update(context.Context, *APIKey) error { panic("unexpected") }
func (s *userVisibleAPIKeyRepoStub) Delete(context.Context, int64) error   { panic("unexpected") }
func (s *userVisibleAPIKeyRepoStub) DeleteWithAudit(context.Context, int64) error {
	panic("unexpected")
}
func (s *userVisibleAPIKeyRepoStub) ListByUserID(context.Context, int64, pagination.PaginationParams, APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error) {
	panic("unexpected")
}
func (s *userVisibleAPIKeyRepoStub) VerifyOwnership(context.Context, int64, []int64) ([]int64, error) {
	return s.validIDs, nil
}
func (s *userVisibleAPIKeyRepoStub) CountByUserID(context.Context, int64) (int64, error) {
	panic("unexpected")
}
func (s *userVisibleAPIKeyRepoStub) ExistsByKey(context.Context, string) (bool, error) {
	panic("unexpected")
}
func (s *userVisibleAPIKeyRepoStub) ListByGroupID(context.Context, int64, pagination.PaginationParams) ([]APIKey, *pagination.PaginationResult, error) {
	panic("unexpected")
}
func (s *userVisibleAPIKeyRepoStub) SearchAPIKeys(context.Context, int64, string, int) ([]APIKey, error) {
	panic("unexpected")
}
func (s *userVisibleAPIKeyRepoStub) ClearGroupIDByGroupID(context.Context, int64) (int64, error) {
	panic("unexpected")
}
func (s *userVisibleAPIKeyRepoStub) UpdateGroupIDByUserAndGroup(context.Context, int64, int64, int64) (int64, error) {
	panic("unexpected")
}
func (s *userVisibleAPIKeyRepoStub) CountByGroupID(context.Context, int64) (int64, error) {
	panic("unexpected")
}
func (s *userVisibleAPIKeyRepoStub) ListKeysByUserID(context.Context, int64) ([]string, error) {
	panic("unexpected")
}
func (s *userVisibleAPIKeyRepoStub) ListKeysByGroupID(context.Context, int64) ([]string, error) {
	panic("unexpected")
}
func (s *userVisibleAPIKeyRepoStub) IncrementQuotaUsed(context.Context, int64, float64) (float64, error) {
	panic("unexpected")
}
func (s *userVisibleAPIKeyRepoStub) UpdateLastUsed(context.Context, int64, time.Time) error {
	panic("unexpected")
}
func (s *userVisibleAPIKeyRepoStub) IncrementRateLimitUsage(context.Context, int64, float64) error {
	panic("unexpected")
}
func (s *userVisibleAPIKeyRepoStub) ResetRateLimitWindows(context.Context, int64) error {
	panic("unexpected")
}
func (s *userVisibleAPIKeyRepoStub) GetRateLimitData(context.Context, int64) (*APIKeyRateLimitData, error) {
	panic("unexpected")
}

func TestAPIKeyServiceGetUserVisibleByIDRejectsFilteredKey(t *testing.T) {
	svc := &APIKeyService{apiKeyRepo: &userVisibleAPIKeyRepoStub{}}

	_, err := svc.GetUserVisibleByID(context.Background(), 42, 99)

	require.ErrorIs(t, err, ErrAPIKeyNotFound)
}

func TestAPIKeyServiceGetUserVisibleByIDReturnsVisibleKey(t *testing.T) {
	repo := &userVisibleAPIKeyRepoStub{
		validIDs: []int64{99},
		key:      &APIKey{ID: 99, UserID: 42, Key: "sk-visible", Source: APIKeySourceUser},
	}
	svc := &APIKeyService{apiKeyRepo: repo}

	key, err := svc.GetUserVisibleByID(context.Background(), 42, 99)

	require.NoError(t, err)
	require.Equal(t, int64(99), key.ID)
	require.Equal(t, APIKeySourceUser, key.Source)
}
