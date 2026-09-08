package admin

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// Operator requests intentionally do not embed administrator request structs.
// Unknown fields fail closed, so upstream additions cannot silently add power.
func bindOperatorJSON(c *gin.Context, out any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
	d := json.NewDecoder(c.Request.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		response.BadRequest(c, "Invalid operator request")
		return false
	}
	if err := d.Decode(new(any)); !errors.Is(err, io.EOF) {
		response.BadRequest(c, "Expected one JSON object")
		return false
	}
	return true
}

func operatorID(c *gin.Context) (int64, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	role, roleOK := middleware.GetUserRoleFromContext(c)
	if !ok || !roleOK || role != service.RoleOperator || subject.UserID <= 0 {
		response.Forbidden(c, "Operator access required")
		return 0, false
	}
	return subject.UserID, true
}

func ownedBy(manager *int64, actor int64) bool {
	return manager != nil && *manager == actor && actor > 0
}

func canContribute(g *service.Group, actor int64) bool {
	return g != nil && g.Status == service.StatusActive && (ownedBy(g.ManagerUserID, actor) || g.AllowOperatorContribution)
}

type operatorAccount struct {
	ID                int64           `json:"id"`
	Name              string          `json:"name"`
	Platform          string          `json:"platform"`
	Type              string          `json:"type"`
	Status            string          `json:"status"`
	ManagerUserID     *int64          `json:"manager_user_id"`
	Concurrency       int             `json:"concurrency"`
	Priority          int             `json:"priority"`
	GroupIDs          []int64         `json:"group_ids"`
	CredentialsStatus map[string]bool `json:"credentials_status"`
}

func operatorAccountView(ctx context.Context, svc service.AdminService, a *service.Account, actor int64) operatorAccount {
	_, status := dto.RedactCredentials(a.Credentials)
	ids := make([]int64, 0, len(a.GroupIDs))
	for _, id := range a.GroupIDs {
		g, err := svc.GetGroup(ctx, id)
		if err == nil && canContribute(g, actor) {
			ids = append(ids, id)
		}
	}
	return operatorAccount{a.ID, a.Name, a.Platform, a.Type, a.Status, a.ManagerUserID, a.Concurrency, a.Priority, ids, status}
}

type operatorGroup struct {
	ID                        int64    `json:"id"`
	Name                      string   `json:"name"`
	Description               string   `json:"description"`
	Platform                  string   `json:"platform"`
	Status                    string   `json:"status"`
	ManagerUserID             *int64   `json:"manager_user_id"`
	AllowOperatorContribution bool     `json:"allow_operator_contribution"`
	SubscriptionType          string   `json:"subscription_type"`
	DailyLimitUSD             *float64 `json:"daily_limit_usd"`
	WeeklyLimitUSD            *float64 `json:"weekly_limit_usd"`
	MonthlyLimitUSD           *float64 `json:"monthly_limit_usd"`
}

func operatorGroupView(g *service.Group) operatorGroup {
	return operatorGroup{g.ID, g.Name, g.Description, g.Platform, g.Status, g.ManagerUserID, g.AllowOperatorContribution, g.SubscriptionType, g.DailyLimitUSD, g.WeeklyLimitUSD, g.MonthlyLimitUSD}
}

func (h *AccountHandler) OperatorList(c *gin.Context) {
	actor, ok := operatorID(c)
	if !ok {
		return
	}
	page, size := response.ParsePagination(c)
	ctx := service.WithResourceManagerScope(c.Request.Context(), actor, false)
	accounts, total, err := h.adminService.ListAccounts(ctx, page, size, "", "", "", strings.TrimSpace(c.Query("search")), 0, "", "id", "desc")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items := make([]operatorAccount, 0, len(accounts))
	for i := range accounts {
		items = append(items, operatorAccountView(ctx, h.adminService, &accounts[i], actor))
	}
	response.Paginated(c, items, total, page, size)
}

func (h *GroupHandler) OperatorList(c *gin.Context) {
	actor, ok := operatorID(c)
	if !ok {
		return
	}
	page, size := response.ParsePagination(c)
	ctx := service.WithResourceManagerScope(c.Request.Context(), actor, false)
	groups, total, err := h.adminService.ListGroups(ctx, page, size, "", "", strings.TrimSpace(c.Query("search")), nil, "id", "desc")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items := make([]operatorGroup, 0, len(groups))
	for i := range groups {
		items = append(items, operatorGroupView(&groups[i]))
	}
	response.Paginated(c, items, total, page, size)
}

func (h *GroupHandler) OperatorOptions(c *gin.Context) {
	actor, ok := operatorID(c)
	if !ok {
		return
	}
	ctx := service.WithResourceManagerScope(c.Request.Context(), actor, true)
	items := make([]gin.H, 0)
	for page := 1; ; page++ {
		groups, total, err := h.adminService.ListGroups(ctx, page, 200, "", service.StatusActive, "", nil, "id", "asc")
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		for _, g := range groups {
			items = append(items, gin.H{"id": g.ID, "name": g.Name, "platform": g.Platform, "manager_user_id": g.ManagerUserID, "allow_operator_contribution": g.AllowOperatorContribution})
		}
		if len(groups) < 200 || int64(page*200) >= total {
			break
		}
	}
	response.Success(c, items)
}

func (h *AccountHandler) OperatorProxyOptions(c *gin.Context) {
	if _, ok := operatorID(c); !ok {
		return
	}
	proxies, err := h.adminService.GetAllProxies(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items := make([]gin.H, 0)
	for _, p := range proxies {
		if p.Status == service.StatusActive {
			items = append(items, gin.H{"id": p.ID, "name": p.Name})
		}
	}
	response.Success(c, items)
}

// Each supplied group must be visible and active. Hidden existing bindings are
// retained, never round-tripped through the browser. The repository rechecks
// ownership and binding policy under its transaction lock before writing.
func operatorBindings(ctx context.Context, svc service.AdminService, actor int64, platform string, requested []int64, existing []int64) ([]int64, error) {
	if len(requested) > 100 {
		return nil, errors.New("too many group bindings")
	}
	seen := map[int64]bool{}
	ids := make([]int64, 0, len(requested)+len(existing))
	for _, id := range requested {
		g, err := svc.GetGroup(ctx, id)
		if err != nil || !canContribute(g, actor) || g.Platform != platform {
			return nil, errors.New("group is not available for contribution")
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	for _, id := range existing {
		g, err := svc.GetGroup(ctx, id)
		if err != nil {
			return nil, errors.New("cannot validate existing binding")
		}
		if !canContribute(g, actor) && !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	return ids, nil
}

func operatorProxy(ctx context.Context, svc service.AdminService, id *int64) error {
	if id == nil {
		return nil
	}
	if *id <= 0 {
		return errors.New("invalid proxy")
	}
	p, err := svc.GetProxy(ctx, *id)
	if err != nil || p.Status != service.StatusActive {
		return errors.New("proxy is not available")
	}
	return nil
}

type operatorCreateAccountRequest struct {
	Name        string  `json:"name"`
	Platform    string  `json:"platform"`
	Type        string  `json:"type"`
	APIKey      string  `json:"api_key"`
	ProxyID     *int64  `json:"proxy_id"`
	GroupIDs    []int64 `json:"group_ids"`
	Concurrency int     `json:"concurrency"`
	Priority    int     `json:"priority"`
}

func validOperatorName(name string) bool { return strings.TrimSpace(name) != "" && len(name) <= 100 }
func validOperatorPlatform(platform string) bool {
	return platform == service.PlatformOpenAI || platform == service.PlatformAnthropic || platform == service.PlatformGemini
}
func normalizeOperatorCapacity(concurrency, priority int) (int, bool) {
	if concurrency == 0 {
		concurrency = 1
	}
	return concurrency, concurrency > 0 && concurrency <= 100 && priority >= 0 && priority <= 1000
}

func (h *AccountHandler) OperatorCreate(c *gin.Context) {
	actor, ok := operatorID(c)
	if !ok {
		return
	}
	var req operatorCreateAccountRequest
	if !bindOperatorJSON(c, &req) {
		return
	}
	concurrency, valid := normalizeOperatorCapacity(req.Concurrency, req.Priority)
	if !valid || !validOperatorName(req.Name) || !validOperatorPlatform(req.Platform) || req.Type != service.AccountTypeAPIKey || strings.TrimSpace(req.APIKey) == "" {
		response.BadRequest(c, "Invalid account fields")
		return
	}
	ctx := service.WithResourceManagerScope(c.Request.Context(), actor, false)
	groups, err := operatorBindings(ctx, h.adminService, actor, req.Platform, req.GroupIDs, nil)
	if err != nil {
		response.Forbidden(c, err.Error())
		return
	}
	if err = operatorProxy(ctx, h.adminService, req.ProxyID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	account, err := h.adminService.CreateAccount(ctx, &service.CreateAccountInput{Name: strings.TrimSpace(req.Name), Platform: req.Platform, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"api_key": req.APIKey}, ProxyID: req.ProxyID, GroupIDs: groups, Concurrency: concurrency, Priority: req.Priority, ManagerUserID: &actor, SkipDefaultGroupBind: true})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, operatorAccountView(ctx, h.adminService, account, actor))
}

func (h *AccountHandler) OperatorUpdate(c *gin.Context) {
	actor, ok := operatorID(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	var req struct {
		Name        string   `json:"name"`
		GroupIDs    *[]int64 `json:"group_ids"`
		Concurrency *int     `json:"concurrency"`
		Priority    *int     `json:"priority"`
	}
	if !bindOperatorJSON(c, &req) {
		return
	}
	if (req.Name != "" && !validOperatorName(req.Name)) || (req.Concurrency != nil && (*req.Concurrency < 1 || *req.Concurrency > 100)) || (req.Priority != nil && (*req.Priority < 0 || *req.Priority > 1000)) {
		response.BadRequest(c, "Invalid account fields")
		return
	}
	ctx := service.WithResourceManagerScope(c.Request.Context(), actor, false)
	account, err := h.adminService.GetAccount(ctx, id)
	if err != nil || !ownedBy(account.ManagerUserID, actor) {
		response.NotFound(c, "Account not found")
		return
	}
	input := &service.UpdateAccountInput{Name: strings.TrimSpace(req.Name), Concurrency: req.Concurrency, Priority: req.Priority}
	if req.GroupIDs != nil {
		groups, e := operatorBindings(ctx, h.adminService, actor, account.Platform, *req.GroupIDs, account.GroupIDs)
		if e != nil {
			response.Forbidden(c, e.Error())
			return
		}
		input.GroupIDs = &groups
	}
	updated, err := h.adminService.UpdateAccount(ctx, id, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, operatorAccountView(ctx, h.adminService, updated, actor))
}

func (h *GroupHandler) OperatorCreate(c *gin.Context) {
	actor, ok := operatorID(c)
	if !ok {
		return
	}
	if h.cfg != nil && h.cfg.RunMode == config.RunModeSimple {
		response.Forbidden(c, "Subscription groups require standard mode")
		return
	}
	var req struct {
		Name             string   `json:"name"`
		Description      string   `json:"description"`
		Platform         string   `json:"platform"`
		SubscriptionType string   `json:"subscription_type"`
		Daily            *float64 `json:"daily_limit_usd"`
		Weekly           *float64 `json:"weekly_limit_usd"`
		Monthly          *float64 `json:"monthly_limit_usd"`
	}
	if !bindOperatorJSON(c, &req) {
		return
	}
	if !validOperatorName(req.Name) || len(req.Description) > 2000 || !validOperatorPlatform(req.Platform) || (req.SubscriptionType != "" && req.SubscriptionType != service.SubscriptionTypeSubscription) {
		response.BadRequest(c, "Invalid subscription group fields")
		return
	}
	for _, limit := range []*float64{req.Daily, req.Weekly, req.Monthly} {
		if limit != nil && *limit < 0 {
			response.BadRequest(c, "Invalid quota")
			return
		}
	}
	ctx := service.WithResourceManagerScope(c.Request.Context(), actor, false)
	group, err := h.adminService.CreateGroup(ctx, &service.CreateGroupInput{Name: strings.TrimSpace(req.Name), Description: req.Description, Platform: req.Platform, SubscriptionType: service.SubscriptionTypeSubscription, IsExclusive: true, RateMultiplier: 1, ManagerUserID: &actor, DailyLimitUSD: req.Daily, WeeklyLimitUSD: req.Weekly, MonthlyLimitUSD: req.Monthly})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, operatorGroupView(group))
}

func (h *GroupHandler) OperatorUpdate(c *gin.Context) {
	actor, ok := operatorID(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid group ID")
		return
	}
	var req struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}
	if !bindOperatorJSON(c, &req) {
		return
	}
	if (req.Name != "" && !validOperatorName(req.Name)) || (req.Description != nil && len(*req.Description) > 2000) {
		response.BadRequest(c, "Invalid group fields")
		return
	}
	ctx := service.WithResourceManagerScope(c.Request.Context(), actor, false)
	group, err := h.adminService.GetGroup(ctx, id)
	if err != nil || !ownedBy(group.ManagerUserID, actor) {
		response.NotFound(c, "Group not found")
		return
	}
	input := &service.UpdateGroupInput{Name: strings.TrimSpace(req.Name), Description: req.Description}
	updated, err := h.adminService.UpdateGroup(ctx, id, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, operatorGroupView(updated))
}

func (h *AccountHandler) OperatorOpenAIAuthURL(c *gin.Context) {
	actor, ok := operatorID(c)
	if !ok {
		return
	}
	var req struct {
		ProxyID *int64 `json:"proxy_id"`
	}
	if !bindOperatorJSON(c, &req) {
		return
	}
	if err := operatorProxy(c.Request.Context(), h.adminService, req.ProxyID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	result, err := h.openaiOAuthService.GenerateOperatorAuthURL(c.Request.Context(), actor, req.ProxyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AccountHandler) OperatorOpenAICreate(c *gin.Context) {
	actor, ok := operatorID(c)
	if !ok {
		return
	}
	var req struct {
		SessionID   string  `json:"session_id"`
		Code        string  `json:"code"`
		State       string  `json:"state"`
		Name        string  `json:"name"`
		ProxyID     *int64  `json:"proxy_id"`
		GroupIDs    []int64 `json:"group_ids"`
		Concurrency int     `json:"concurrency"`
		Priority    int     `json:"priority"`
	}
	if !bindOperatorJSON(c, &req) {
		return
	}
	concurrency, valid := normalizeOperatorCapacity(req.Concurrency, req.Priority)
	if !valid || !validOperatorName(req.Name) || req.SessionID == "" || req.Code == "" || req.State == "" {
		response.BadRequest(c, "Invalid OAuth account fields")
		return
	}
	ctx := service.WithResourceManagerScope(c.Request.Context(), actor, false)
	groups, err := operatorBindings(ctx, h.adminService, actor, service.PlatformOpenAI, req.GroupIDs, nil)
	if err != nil {
		response.Forbidden(c, err.Error())
		return
	}
	if err = operatorProxy(ctx, h.adminService, req.ProxyID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	token, err := h.openaiOAuthService.ExchangeOperatorCode(ctx, actor, &service.OpenAIExchangeCodeInput{SessionID: req.SessionID, Code: req.Code, State: req.State, ProxyID: req.ProxyID})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	account, err := h.adminService.CreateAccount(ctx, &service.CreateAccountInput{Name: strings.TrimSpace(req.Name), Platform: service.PlatformOpenAI, Type: service.AccountTypeOAuth, Credentials: h.openaiOAuthService.BuildAccountCredentials(token), ProxyID: req.ProxyID, GroupIDs: groups, Concurrency: concurrency, Priority: req.Priority, ManagerUserID: &actor, SkipDefaultGroupBind: true})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, operatorAccountView(ctx, h.adminService, account, actor))
}
