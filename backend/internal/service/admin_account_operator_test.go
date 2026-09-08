//go:build unit

package service

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCreateAccountOperatorUsesAtomicPersistence(t *testing.T) {
	repo := newDuplicateAccountRepoStub()
	svc := &adminServiceImpl{accountRepo: repo, accountDuplicateRepo: repo}
	owner := int64(42)
	ctx := WithResourceManagerScope(context.Background(), owner, false)
	input := &CreateAccountInput{Name: "operator", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, ManagerUserID: &owner, Credentials: map[string]any{"api_key": "test"}, SkipDefaultGroupBind: true}
	repo.atomicCreateErr = errors.New("atomic failure")
	_, err := svc.CreateAccount(ctx, input)
	require.ErrorContains(t, err, "atomic failure")
	require.Empty(t, repo.accounts)
	repo.atomicCreateErr = nil
	account, err := svc.CreateAccount(ctx, input)
	require.NoError(t, err)
	require.Equal(t, &owner, account.ManagerUserID)
	require.Contains(t, repo.accountGroupsOf, account.ID)
}
