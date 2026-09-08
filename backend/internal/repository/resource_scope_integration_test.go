//go:build integration

package repository

import (
	"context"
	"fmt"
	"github.com/Wei-Shaw/sub2api/ent/accountgroup"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestResourceManagerPersistenceAndIsolation(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	user := mustCreateUser(t, client, &service.User{})
	other := mustCreateUser(t, client, &service.User{})
	groups := newGroupRepositoryWithSQL(client, tx)
	accounts := newAccountRepositoryWithSQL(client, tx, nil)
	own := &service.Group{Name: "owned", Platform: service.PlatformOpenAI, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard, ManagerUserID: &user.ID}
	public := &service.Group{Name: "contributable", Platform: service.PlatformOpenAI, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard, ManagerUserID: &other.ID, AllowOperatorContribution: true}
	hidden := &service.Group{Name: "private", Platform: service.PlatformOpenAI, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard}
	for _, g := range []*service.Group{own, public, hidden} {
		require.NoError(t, groups.Create(ctx, g))
	}
	scoped := service.WithResourceManagerScope(ctx, user.ID, false)
	params := pagination.PaginationParams{Page: 1, PageSize: 10}
	listed, total, err := groups.ListWithFilters(scoped, params, "", "", "", nil)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.EqualValues(t, 1, total.Total)
	listed, total, err = groups.ListWithFilters(service.WithResourceManagerScope(ctx, user.ID, true), params, "", "", "", nil)
	require.NoError(t, err)
	require.Len(t, listed, 2)
	require.EqualValues(t, 2, total.Total)
	listed, _, err = groups.ListWithFilters(ctx, params, "", "", "", nil)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(listed), 3)
	a := &service.Account{Name: "owned-account", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Credentials: map[string]any{}, Extra: map[string]any{}, Concurrency: 1, ManagerUserID: &user.ID}
	require.NoError(t, accounts.Create(ctx, a))
	read, err := accounts.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.Equal(t, &user.ID, read.ManagerUserID)
	rows, count, err := accounts.ListWithFilters(scoped, params, "", "", "", "", 0, "")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.EqualValues(t, 1, count.Total)
	require.NoError(t, accounts.BindGroups(ctx, a.ID, []int64{hidden.ID}))
	require.NoError(t, accounts.BindGroups(scoped, a.ID, []int64{public.ID}))
	bound, err := accounts.GetGroups(ctx, a.ID)
	require.NoError(t, err)
	require.Len(t, bound, 2)
	require.Error(t, accounts.BindGroups(service.WithResourceManagerScope(ctx, other.ID, false), a.ID, nil))
	own.ManagerUserID = &other.ID
	require.NoError(t, groups.Update(ctx, own))
	own.Name = "must-not-change"
	require.Error(t, groups.Update(scoped, own))
	own.ManagerUserID = nil
	require.NoError(t, groups.Update(ctx, own))
	fetched, err := groups.GetByID(ctx, own.ID)
	require.NoError(t, err)
	require.Nil(t, fetched.ManagerUserID)
}

func TestOperatorUpdatesPreserveConcurrentPolicyAndRollbackBindings(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{})
	accounts := newAccountRepositoryWithSQL(client, integrationDB, nil)
	groups := newGroupRepositoryWithSQL(client, integrationDB)
	// Distinct names avoid existing platform-default resources.
	g := &service.Group{Name: fmt.Sprintf("operator-policy-%d", user.ID), Platform: service.PlatformOpenAI, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard, ManagerUserID: &user.ID}
	require.NoError(t, groups.Create(ctx, g))
	a := &service.Account{Name: "before", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Credentials: map[string]any{}, Extra: map[string]any{}, Concurrency: 1, ManagerUserID: &user.ID}
	require.NoError(t, accounts.Create(ctx, a))
	t.Cleanup(func() {
		_, _ = integrationDB.Exec(`DELETE FROM account_groups WHERE account_id=$1`, a.ID)
		_, _ = integrationDB.Exec(`DELETE FROM accounts WHERE id=$1`, a.ID)
		_, _ = integrationDB.Exec(`DELETE FROM groups WHERE id=$1`, g.ID)
		_, _ = integrationDB.Exec(`DELETE FROM users WHERE id=$1`, user.ID)
	})
	scoped := service.WithResourceManagerScope(ctx, user.ID, false)
	newName := "edited"
	patchCtx := service.WithOperatorResourcePatch(scoped, service.OperatorResourcePatch{Name: &newName})
	// Administrator updates occur after these entity snapshots were loaded.
	_, err := client.Account.UpdateOneID(a.ID).SetConcurrency(13).SetPriority(71).Save(ctx)
	require.NoError(t, err)
	require.NoError(t, accounts.Update(patchCtx, a))
	fresh, err := accounts.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.Equal(t, "edited", fresh.Name)
	require.Equal(t, 13, fresh.Concurrency)
	require.Equal(t, 71, fresh.Priority)
	_, err = client.Group.UpdateOneID(g.ID).SetAllowOperatorContribution(true).SetRateMultiplier(7).Save(ctx)
	require.NoError(t, err)
	require.NoError(t, groups.Update(patchCtx, g))
	freshGroup, err := groups.GetByID(ctx, g.ID)
	require.NoError(t, err)
	require.True(t, freshGroup.AllowOperatorContribution)
	require.Equal(t, float64(7), freshGroup.RateMultiplier)
	// Existing administrator binding must keep its priority after an operator edit.
	_, err = client.AccountGroup.Create().SetAccountID(a.ID).SetGroupID(g.ID).SetPriority(83).Save(ctx)
	require.NoError(t, err)
	_, err = client.Group.UpdateOneID(g.ID).ClearManagerUserID().SetAllowOperatorContribution(false).Save(ctx)
	require.NoError(t, err)
	require.NoError(t, accounts.BindGroups(scoped, a.ID, nil))
	binding, err := client.AccountGroup.Query().Where(accountgroup.AccountIDEQ(a.ID), accountgroup.GroupIDEQ(g.ID)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, 83, binding.Priority)
	// A newly requested group whose platform changed cannot commit even the name edit.
	_, err = client.AccountGroup.Delete().Where(accountgroup.AccountIDEQ(a.ID)).Exec(ctx)
	require.NoError(t, err)
	_, err = client.Group.UpdateOneID(g.ID).SetAllowOperatorContribution(true).SetPlatform(service.PlatformAnthropic).Save(ctx)
	require.NoError(t, err)
	rollbackName := "must rollback"
	err = accounts.RunResourceTransaction(scoped, func(txCtx context.Context) error {
		txCtx = service.WithOperatorResourcePatch(txCtx, service.OperatorResourcePatch{Name: &rollbackName})
		if err := accounts.Update(txCtx, a); err != nil {
			return err
		}
		return accounts.BindGroups(txCtx, a.ID, []int64{g.ID})
	})
	require.Error(t, err)
	fresh, err = accounts.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.Equal(t, "edited", fresh.Name)
}
