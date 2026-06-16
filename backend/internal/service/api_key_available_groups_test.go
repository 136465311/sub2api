package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type availableGroupsUserRepoStub struct {
	UserRepository
	user *User
}

func (s *availableGroupsUserRepoStub) GetByID(context.Context, int64) (*User, error) {
	if s.user == nil {
		return nil, ErrUserNotFound
	}
	clone := *s.user
	clone.AllowedGroups = append([]int64(nil), s.user.AllowedGroups...)
	return &clone, nil
}

type availableGroupsGroupRepoStub struct {
	groups []Group
}

func (s *availableGroupsGroupRepoStub) Create(context.Context, *Group) error {
	panic("unexpected Create call")
}
func (s *availableGroupsGroupRepoStub) GetByID(_ context.Context, id int64) (*Group, error) {
	for i := range s.groups {
		if s.groups[i].ID == id {
			clone := s.groups[i]
			return &clone, nil
		}
	}
	return nil, ErrGroupNotFound
}
func (s *availableGroupsGroupRepoStub) GetByIDLite(context.Context, int64) (*Group, error) {
	panic("unexpected GetByIDLite call")
}
func (s *availableGroupsGroupRepoStub) Update(context.Context, *Group) error {
	panic("unexpected Update call")
}
func (s *availableGroupsGroupRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}
func (s *availableGroupsGroupRepoStub) DeleteCascade(context.Context, int64) ([]int64, error) {
	panic("unexpected DeleteCascade call")
}
func (s *availableGroupsGroupRepoStub) List(context.Context, pagination.PaginationParams) ([]Group, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}
func (s *availableGroupsGroupRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string, *bool) ([]Group, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}
func (s *availableGroupsGroupRepoStub) ListActive(context.Context) ([]Group, error) {
	out := make([]Group, 0, len(s.groups))
	for _, group := range s.groups {
		if group.Status == StatusActive {
			out = append(out, group)
		}
	}
	return out, nil
}
func (s *availableGroupsGroupRepoStub) ListActiveByPlatform(context.Context, string) ([]Group, error) {
	panic("unexpected ListActiveByPlatform call")
}
func (s *availableGroupsGroupRepoStub) ExistsByName(context.Context, string) (bool, error) {
	panic("unexpected ExistsByName call")
}
func (s *availableGroupsGroupRepoStub) GetAccountCount(context.Context, int64) (int64, int64, error) {
	panic("unexpected GetAccountCount call")
}
func (s *availableGroupsGroupRepoStub) DeleteAccountGroupsByGroupID(context.Context, int64) (int64, error) {
	panic("unexpected DeleteAccountGroupsByGroupID call")
}
func (s *availableGroupsGroupRepoStub) GetAccountIDsByGroupIDs(context.Context, []int64) ([]int64, error) {
	panic("unexpected GetAccountIDsByGroupIDs call")
}
func (s *availableGroupsGroupRepoStub) BindAccountsToGroup(context.Context, int64, []int64) error {
	panic("unexpected BindAccountsToGroup call")
}
func (s *availableGroupsGroupRepoStub) UpdateSortOrders(context.Context, []GroupSortOrderUpdate) error {
	panic("unexpected UpdateSortOrders call")
}

type availableGroupsAPIKeyRepoStub struct {
	mu         sync.Mutex
	keys       []APIKey
	createHook func()
}

func (s *availableGroupsAPIKeyRepoStub) Create(_ context.Context, key *APIKey) error {
	if s.createHook != nil {
		s.createHook()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key.ID = int64(len(s.keys) + 1)
	clone := *key
	s.keys = append(s.keys, clone)
	return nil
}
func (s *availableGroupsAPIKeyRepoStub) GetByID(context.Context, int64) (*APIKey, error) {
	panic("unexpected GetByID call")
}
func (s *availableGroupsAPIKeyRepoStub) GetKeyAndOwnerID(context.Context, int64) (string, int64, error) {
	panic("unexpected GetKeyAndOwnerID call")
}
func (s *availableGroupsAPIKeyRepoStub) GetByKey(context.Context, string) (*APIKey, error) {
	panic("unexpected GetByKey call")
}
func (s *availableGroupsAPIKeyRepoStub) GetByKeyForAuth(context.Context, string) (*APIKey, error) {
	panic("unexpected GetByKeyForAuth call")
}
func (s *availableGroupsAPIKeyRepoStub) GetBySourceForUserGroup(_ context.Context, userID int64, groupID *int64, source string) (*APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.keys) - 1; i >= 0; i-- {
		key := s.keys[i]
		if key.UserID != userID || key.Source != source {
			continue
		}
		if !optionalInt64Equal(key.GroupID, groupID) {
			continue
		}
		clone := key
		return &clone, nil
	}
	return nil, ErrAPIKeyNotFound
}
func (s *availableGroupsAPIKeyRepoStub) Update(context.Context, *APIKey) error {
	panic("unexpected Update call")
}
func (s *availableGroupsAPIKeyRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}
func (s *availableGroupsAPIKeyRepoStub) DeleteWithAudit(context.Context, int64) error {
	panic("unexpected DeleteWithAudit call")
}
func (s *availableGroupsAPIKeyRepoStub) ListByUserID(_ context.Context, userID int64, _ pagination.PaginationParams, filters APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]APIKey, 0, len(s.keys))
	for _, key := range s.keys {
		if key.UserID != userID || key.Source != APIKeySourceUser {
			continue
		}
		if filters.Status != "" && key.Status != filters.Status {
			continue
		}
		out = append(out, key)
	}
	return out, &pagination.PaginationResult{Total: int64(len(out)), Page: 1, PageSize: 10000, Pages: 1}, nil
}
func (s *availableGroupsAPIKeyRepoStub) VerifyOwnership(context.Context, int64, []int64) ([]int64, error) {
	panic("unexpected VerifyOwnership call")
}
func (s *availableGroupsAPIKeyRepoStub) CountByUserID(context.Context, int64) (int64, error) {
	panic("unexpected CountByUserID call")
}
func (s *availableGroupsAPIKeyRepoStub) ExistsByKey(context.Context, string) (bool, error) {
	return false, nil
}
func (s *availableGroupsAPIKeyRepoStub) ListByGroupID(context.Context, int64, pagination.PaginationParams) ([]APIKey, *pagination.PaginationResult, error) {
	panic("unexpected ListByGroupID call")
}
func (s *availableGroupsAPIKeyRepoStub) SearchAPIKeys(context.Context, int64, string, int) ([]APIKey, error) {
	panic("unexpected SearchAPIKeys call")
}
func (s *availableGroupsAPIKeyRepoStub) ClearGroupIDByGroupID(context.Context, int64) (int64, error) {
	panic("unexpected ClearGroupIDByGroupID call")
}
func (s *availableGroupsAPIKeyRepoStub) UpdateGroupIDByUserAndGroup(context.Context, int64, int64, int64) (int64, error) {
	panic("unexpected UpdateGroupIDByUserAndGroup call")
}
func (s *availableGroupsAPIKeyRepoStub) CountByGroupID(context.Context, int64) (int64, error) {
	panic("unexpected CountByGroupID call")
}
func (s *availableGroupsAPIKeyRepoStub) ListKeysByUserID(context.Context, int64) ([]string, error) {
	panic("unexpected ListKeysByUserID call")
}
func (s *availableGroupsAPIKeyRepoStub) ListKeysByGroupID(context.Context, int64) ([]string, error) {
	panic("unexpected ListKeysByGroupID call")
}
func (s *availableGroupsAPIKeyRepoStub) IncrementQuotaUsed(context.Context, int64, float64) (float64, error) {
	panic("unexpected IncrementQuotaUsed call")
}
func (s *availableGroupsAPIKeyRepoStub) UpdateLastUsed(context.Context, int64, time.Time) error {
	panic("unexpected UpdateLastUsed call")
}
func (s *availableGroupsAPIKeyRepoStub) IncrementRateLimitUsage(context.Context, int64, float64) error {
	panic("unexpected IncrementRateLimitUsage call")
}
func (s *availableGroupsAPIKeyRepoStub) ResetRateLimitWindows(context.Context, int64) error {
	panic("unexpected ResetRateLimitWindows call")
}
func (s *availableGroupsAPIKeyRepoStub) GetRateLimitData(context.Context, int64) (*APIKeyRateLimitData, error) {
	panic("unexpected GetRateLimitData call")
}

func optionalInt64Equal(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

type availableGroupsSubRepoStub struct {
	active []UserSubscription
}

func (s *availableGroupsSubRepoStub) Create(context.Context, *UserSubscription) error {
	panic("unexpected Create call")
}
func (s *availableGroupsSubRepoStub) GetByID(context.Context, int64) (*UserSubscription, error) {
	panic("unexpected GetByID call")
}
func (s *availableGroupsSubRepoStub) GetByUserIDAndGroupID(context.Context, int64, int64) (*UserSubscription, error) {
	panic("unexpected GetByUserIDAndGroupID call")
}
func (s *availableGroupsSubRepoStub) GetActiveByUserIDAndGroupID(context.Context, int64, int64) (*UserSubscription, error) {
	panic("unexpected GetActiveByUserIDAndGroupID call")
}
func (s *availableGroupsSubRepoStub) Update(context.Context, *UserSubscription) error {
	panic("unexpected Update call")
}
func (s *availableGroupsSubRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}
func (s *availableGroupsSubRepoStub) ListByUserID(context.Context, int64) ([]UserSubscription, error) {
	panic("unexpected ListByUserID call")
}
func (s *availableGroupsSubRepoStub) ListActiveByUserID(_ context.Context, userID int64) ([]UserSubscription, error) {
	out := make([]UserSubscription, 0, len(s.active))
	for _, sub := range s.active {
		if sub.UserID == userID {
			out = append(out, sub)
		}
	}
	return out, nil
}
func (s *availableGroupsSubRepoStub) ListByGroupID(context.Context, int64, pagination.PaginationParams) ([]UserSubscription, *pagination.PaginationResult, error) {
	panic("unexpected ListByGroupID call")
}
func (s *availableGroupsSubRepoStub) List(context.Context, pagination.PaginationParams, *int64, *int64, string, string, string, string) ([]UserSubscription, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}
func (s *availableGroupsSubRepoStub) ExistsByUserIDAndGroupID(context.Context, int64, int64) (bool, error) {
	panic("unexpected ExistsByUserIDAndGroupID call")
}
func (s *availableGroupsSubRepoStub) ExtendExpiry(context.Context, int64, time.Time) error {
	panic("unexpected ExtendExpiry call")
}
func (s *availableGroupsSubRepoStub) UpdateStatus(context.Context, int64, string) error {
	panic("unexpected UpdateStatus call")
}
func (s *availableGroupsSubRepoStub) UpdateNotes(context.Context, int64, string) error {
	panic("unexpected UpdateNotes call")
}
func (s *availableGroupsSubRepoStub) ActivateWindows(context.Context, int64, time.Time) error {
	panic("unexpected ActivateWindows call")
}
func (s *availableGroupsSubRepoStub) ResetDailyUsage(context.Context, int64, time.Time) error {
	panic("unexpected ResetDailyUsage call")
}
func (s *availableGroupsSubRepoStub) ResetWeeklyUsage(context.Context, int64, time.Time) error {
	panic("unexpected ResetWeeklyUsage call")
}
func (s *availableGroupsSubRepoStub) ResetMonthlyUsage(context.Context, int64, time.Time) error {
	panic("unexpected ResetMonthlyUsage call")
}
func (s *availableGroupsSubRepoStub) IncrementUsage(context.Context, int64, float64) error {
	panic("unexpected IncrementUsage call")
}
func (s *availableGroupsSubRepoStub) BatchUpdateExpiredStatus(context.Context) (int64, error) {
	panic("unexpected BatchUpdateExpiredStatus call")
}

func TestAPIKeyServiceGetAvailableGroupsIncludesExistingActiveUserKeyGroup(t *testing.T) {
	exclusiveID := int64(12)
	svc := newAvailableGroupsTestService(
		&User{ID: 7, Role: RoleUser, Status: StatusActive},
		[]Group{
			{ID: 1, Name: "public", Status: StatusActive, SubscriptionType: SubscriptionTypeStandard},
			{ID: exclusiveID, Name: "chatgpt-plus", Status: StatusActive, IsExclusive: true, SubscriptionType: SubscriptionTypeStandard, Platform: PlatformOpenAI},
		},
		[]APIKey{{ID: 55, UserID: 7, Source: APIKeySourceUser, Status: StatusActive, GroupID: &exclusiveID}},
		nil,
	)

	groups, err := svc.GetAvailableGroups(context.Background(), 7)

	require.NoError(t, err)
	require.ElementsMatch(t, []string{"public", "chatgpt-plus"}, groupNames(groups))
}

func TestAPIKeyServiceGetAvailableGroupsDoesNotUseInactiveKeyGroup(t *testing.T) {
	exclusiveID := int64(12)
	svc := newAvailableGroupsTestService(
		&User{ID: 7, Role: RoleUser, Status: StatusActive},
		[]Group{{ID: exclusiveID, Name: "chatgpt-plus", Status: StatusActive, IsExclusive: true, SubscriptionType: SubscriptionTypeStandard, Platform: PlatformOpenAI}},
		[]APIKey{{ID: 55, UserID: 7, Source: APIKeySourceUser, Status: StatusAPIKeyDisabled, GroupID: &exclusiveID}},
		nil,
	)

	groups, err := svc.GetAvailableGroups(context.Background(), 7)

	require.NoError(t, err)
	require.Empty(t, groups)
}

func TestAPIKeyServiceGetAvailableGroupsDoesNotUseExistingKeyForSubscriptionGroup(t *testing.T) {
	groupID := int64(30)
	svc := newAvailableGroupsTestService(
		&User{ID: 7, Role: RoleUser, Status: StatusActive},
		[]Group{{ID: groupID, Name: "sub", Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription}},
		[]APIKey{{ID: 55, UserID: 7, Source: APIKeySourceUser, Status: StatusActive, GroupID: &groupID}},
		nil,
	)

	groups, err := svc.GetAvailableGroups(context.Background(), 7)

	require.NoError(t, err)
	require.Empty(t, groups)
}

func TestAPIKeyServiceGetAvailableGroupsIncludesAdminActiveGroups(t *testing.T) {
	svc := newAvailableGroupsTestService(
		&User{ID: 1, Role: RoleAdmin, Status: StatusActive},
		[]Group{
			{ID: 12, Name: "chatgpt-plus", Status: StatusActive, IsExclusive: true, SubscriptionType: SubscriptionTypeStandard},
			{ID: 30, Name: "sub", Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription},
		},
		nil,
		nil,
	)

	groups, err := svc.GetAvailableGroups(context.Background(), 1)

	require.NoError(t, err)
	require.ElementsMatch(t, []string{"chatgpt-plus", "sub"}, groupNames(groups))
}

func TestAPIKeyServiceGetOrCreateUserAIInternalKeyAllowsExistingUserKeyGroup(t *testing.T) {
	groupID := int64(12)
	keyRepo := &availableGroupsAPIKeyRepoStub{
		keys: []APIKey{{ID: 55, UserID: 7, Source: APIKeySourceUser, Status: StatusActive, GroupID: &groupID}},
	}
	svc := NewAPIKeyService(
		keyRepo,
		&availableGroupsUserRepoStub{user: &User{ID: 7, Role: RoleUser, Status: StatusActive}},
		&availableGroupsGroupRepoStub{groups: []Group{{ID: groupID, Name: "chatgpt-plus", Status: StatusActive, IsExclusive: true, SubscriptionType: SubscriptionTypeStandard}}},
		&availableGroupsSubRepoStub{},
		nil,
		nil,
		&config.Config{},
	)

	key, err := svc.GetOrCreateUserAIInternalKey(context.Background(), 7, &groupID)

	require.NoError(t, err)
	require.NotNil(t, key)
	require.Equal(t, APIKeySourceUserAI, key.Source)
	require.Equal(t, &groupID, key.GroupID)
}

func TestAPIKeyServiceGetOrCreateUserAIInternalKeyReusesExistingHiddenKey(t *testing.T) {
	groupID := int64(12)
	keyRepo := &availableGroupsAPIKeyRepoStub{
		keys: []APIKey{
			{ID: 55, UserID: 7, Key: "manual-key", Source: APIKeySourceUser, Status: StatusActive, GroupID: &groupID},
			{ID: 56, UserID: 7, Key: "hidden-key", Source: APIKeySourceUserAI, Status: StatusActive, GroupID: &groupID},
		},
	}
	svc := NewAPIKeyService(
		keyRepo,
		&availableGroupsUserRepoStub{user: &User{ID: 7, Role: RoleUser, Status: StatusActive}},
		&availableGroupsGroupRepoStub{groups: []Group{{ID: groupID, Name: "chatgpt-plus", Status: StatusActive, SubscriptionType: SubscriptionTypeStandard}}},
		&availableGroupsSubRepoStub{},
		nil,
		nil,
		&config.Config{},
	)

	key, err := svc.GetOrCreateUserAIInternalKey(context.Background(), 7, &groupID)

	require.NoError(t, err)
	require.Equal(t, int64(56), key.ID)
	require.Equal(t, "hidden-key", key.Key)
	require.Len(t, keyRepo.keys, 2)
}

func TestAPIKeyServiceGetOrCreateUserAIInternalKeySingleflightsConcurrentCreates(t *testing.T) {
	groupID := int64(12)
	releaseCreate := make(chan struct{})
	var hookOnce sync.Once
	keyRepo := &availableGroupsAPIKeyRepoStub{
		createHook: func() {
			hookOnce.Do(func() {
				<-releaseCreate
			})
		},
	}
	svc := NewAPIKeyService(
		keyRepo,
		&availableGroupsUserRepoStub{user: &User{ID: 7, Role: RoleUser, Status: StatusActive}},
		&availableGroupsGroupRepoStub{groups: []Group{{ID: groupID, Name: "chatgpt-plus", Status: StatusActive, SubscriptionType: SubscriptionTypeStandard}}},
		&availableGroupsSubRepoStub{},
		nil,
		nil,
		&config.Config{},
	)

	const callers = 8
	start := make(chan struct{})
	results := make(chan *APIKey, callers)
	errs := make(chan error, callers)
	for i := 0; i < callers; i++ {
		go func() {
			<-start
			key, err := svc.GetOrCreateUserAIInternalKey(context.Background(), 7, &groupID)
			if err != nil {
				errs <- err
				return
			}
			results <- key
		}()
	}

	close(start)
	close(releaseCreate)

	for i := 0; i < callers; i++ {
		select {
		case err := <-errs:
			require.NoError(t, err)
		case key := <-results:
			require.Equal(t, int64(1), key.ID)
			require.Equal(t, APIKeySourceUserAI, key.Source)
			require.Equal(t, &groupID, key.GroupID)
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for concurrent internal key create")
		}
	}
	require.Len(t, keyRepo.keys, 1)
}

func newAvailableGroupsTestService(user *User, groups []Group, keys []APIKey, subs []UserSubscription) *APIKeyService {
	return NewAPIKeyService(
		&availableGroupsAPIKeyRepoStub{keys: keys},
		&availableGroupsUserRepoStub{user: user},
		&availableGroupsGroupRepoStub{groups: groups},
		&availableGroupsSubRepoStub{active: subs},
		nil,
		nil,
		&config.Config{},
	)
}

func groupNames(groups []Group) []string {
	out := make([]string, 0, len(groups))
	for _, group := range groups {
		out = append(out, group.Name)
	}
	return out
}
