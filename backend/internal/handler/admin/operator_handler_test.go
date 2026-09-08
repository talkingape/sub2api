package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type operatorServiceStub struct {
	service.AdminService
	groups  map[int64]*service.Group
	account *service.Account
	create  func(context.Context, *service.CreateAccountInput) (*service.Account, error)
	update  func(context.Context, int64, *service.UpdateAccountInput) (*service.Account, error)
}

func (s *operatorServiceStub) GetGroup(_ context.Context, id int64) (*service.Group, error) {
	g, ok := s.groups[id]
	if !ok {
		return nil, service.ErrGroupNotFound
	}
	return g, nil
}
func (s *operatorServiceStub) GetAccount(_ context.Context, id int64) (*service.Account, error) {
	if s.account == nil || s.account.ID != id {
		return nil, service.ErrAccountNotFound
	}
	return s.account, nil
}
func (s *operatorServiceStub) CreateAccount(ctx context.Context, in *service.CreateAccountInput) (*service.Account, error) {
	return s.create(ctx, in)
}
func (s *operatorServiceStub) UpdateAccount(ctx context.Context, id int64, in *service.UpdateAccountInput) (*service.Account, error) {
	return s.update(ctx, id, in)
}

func operatorRequest(t *testing.T, fn gin.HandlerFunc, method, path, body string, actor int64) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: actor})
		c.Set(string(middleware.ContextKeyUserRole), service.RoleOperator)
	})
	r.Handle(method, path, fn)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, strings.ReplaceAll(path, ":id", "7"), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestOperatorBindingsPreservePrivateAndRejectForeignResources(t *testing.T) {
	actor, other := int64(10), int64(20)
	s := &operatorServiceStub{groups: map[int64]*service.Group{
		1: {ID: 1, Platform: service.PlatformOpenAI, Status: service.StatusActive, ManagerUserID: &actor},
		2: {ID: 2, Platform: service.PlatformOpenAI, Status: service.StatusActive, ManagerUserID: &other, AllowOperatorContribution: true},
		3: {ID: 3, Platform: service.PlatformOpenAI, Status: service.StatusActive, ManagerUserID: &other},
		4: {ID: 4, Platform: service.PlatformOpenAI, Status: service.StatusDisabled, ManagerUserID: &actor},
		5: {ID: 5, Platform: service.PlatformGemini, Status: service.StatusActive, ManagerUserID: &actor},
	}}
	ids, err := operatorBindings(context.Background(), s, actor, service.PlatformOpenAI, []int64{1, 2, 2}, []int64{3})
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2, 3}, ids)
	for _, id := range []int64{3, 4, 5, 999} {
		_, err = operatorBindings(context.Background(), s, actor, service.PlatformOpenAI, []int64{id}, nil)
		require.Error(t, err)
	}
	ids, err = operatorBindings(context.Background(), s, actor, service.PlatformOpenAI, nil, []int64{1, 2, 3})
	require.NoError(t, err)
	require.Equal(t, []int64{3}, ids)
}

func TestOperatorCreateForcesOwnershipAndAvoidsDefaultGroup(t *testing.T) {
	actor := int64(10)
	calls := 0
	s := &operatorServiceStub{groups: map[int64]*service.Group{1: {ID: 1, Platform: service.PlatformOpenAI, Status: service.StatusActive, AllowOperatorContribution: true}}}
	s.create = func(ctx context.Context, in *service.CreateAccountInput) (*service.Account, error) {
		calls++
		scope, ok := service.ResourceManagerScopeFromContext(ctx)
		require.True(t, ok)
		require.Equal(t, actor, scope.UserID)
		require.Equal(t, &actor, in.ManagerUserID)
		require.True(t, in.SkipDefaultGroupBind)
		require.Equal(t, []int64{1}, in.GroupIDs)
		require.Equal(t, map[string]any{"api_key": "sensitive-test-value"}, in.Credentials)
		return &service.Account{ID: 7, Name: in.Name, Platform: in.Platform, Type: in.Type, ManagerUserID: in.ManagerUserID, Credentials: in.Credentials, GroupIDs: in.GroupIDs}, nil
	}
	h := &AccountHandler{adminService: s}
	w := operatorRequest(t, h.OperatorCreate, "POST", "/accounts", `{"name":"mine","platform":"openai","type":"apikey","api_key":"sensitive-test-value","group_ids":[1]}`, actor)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, 1, calls)
	require.NotContains(t, w.Body.String(), "sensitive-test-value")
	for _, extra := range []string{`"manager_user_id":20`, `"credentials":{"base_url":"http://127.0.0.1"}`, `"extra":{}`, `"skip_default_group_bind":false`} {
		w = operatorRequest(t, h.OperatorCreate, "POST", "/accounts", `{"name":"mine","platform":"openai","type":"apikey","api_key":"x",`+extra+`}`, actor)
		require.Equal(t, http.StatusBadRequest, w.Code)
	}
	require.Equal(t, 1, calls)
}

func TestOperatorUpdateRejectsForeignOwnerAndPrivilegedFields(t *testing.T) {
	owner := int64(20)
	calls := 0
	s := &operatorServiceStub{account: &service.Account{ID: 7, ManagerUserID: &owner}}
	s.update = func(context.Context, int64, *service.UpdateAccountInput) (*service.Account, error) {
		calls++
		return s.account, nil
	}
	h := &AccountHandler{adminService: s}
	w := operatorRequest(t, h.OperatorUpdate, "PUT", "/accounts/:id", `{"name":"stolen"}`, 10)
	require.Equal(t, http.StatusNotFound, w.Code)
	for _, body := range []string{`{"manager_user_id":10}`, `{"credentials":{"api_key":"replace"}}`, `{"status":"disabled"}`, `{"proxy_id":1}`} {
		w = operatorRequest(t, h.OperatorUpdate, "PUT", "/accounts/:id", body, owner)
		require.Equal(t, http.StatusBadRequest, w.Code)
	}
	require.Zero(t, calls)
}

func TestOperatorGroupUpdateDoesNotExposeSubscriptionOrContributionPolicy(t *testing.T) {
	h := &GroupHandler{}
	for _, body := range []string{`{"allow_operator_contribution":true}`, `{"manager_user_id":10}`, `{"daily_limit_usd":999}`, `{"subscription_type":"standard"}`, `{"copy_accounts_from_group_ids":[1]}`, `{"fallback_group_id":1}`} {
		w := operatorRequest(t, h.OperatorUpdate, "PUT", "/groups/:id", body, 10)
		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	}
}

func TestOperatorAccountProjectionOmitsHiddenBindingsAndSecrets(t *testing.T) {
	actor, other := int64(10), int64(20)
	s := &operatorServiceStub{groups: map[int64]*service.Group{1: {ID: 1, Status: service.StatusActive, ManagerUserID: &actor}, 2: {ID: 2, Status: service.StatusActive, ManagerUserID: &other}}}
	a := &service.Account{ID: 7, ManagerUserID: &actor, GroupIDs: []int64{1, 2}, Credentials: map[string]any{"access_token": "private-token", "refresh_token": "private-refresh"}, Proxy: &service.Proxy{Host: "private-host"}}
	v := operatorAccountView(context.Background(), s, a, actor)
	require.Equal(t, []int64{1}, v.GroupIDs)
	b, err := json.Marshal(v)
	require.NoError(t, err)
	require.NotContains(t, string(b), "private-")
}
