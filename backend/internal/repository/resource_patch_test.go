package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOperatorPatchDoesNotWriteStaleEntityFields(t *testing.T) {
	for _, entity := range []string{"account", "group"} {
		t.Run(entity, func(t *testing.T) {
			sentinel := errors.New("checked write")
			matcher := sqlmock.QueryMatcherFunc(func(_, actual string) error {
				if !strings.HasPrefix(actual, "UPDATE ") {
					return fmt.Errorf("expected update: %s", actual)
				}
				parts := strings.SplitN(actual, " WHERE ", 2)
				if len(parts) != 2 || !strings.Contains(parts[1], `"manager_user_id" =`) {
					return fmt.Errorf("missing owner guard: %s", actual)
				}
				for _, field := range []string{"manager_user_id", "allow_operator_contribution", "concurrency", "priority", "credentials", "status", "rate_multiplier", "description", "daily_limit_usd"} {
					if strings.Contains(parts[0], `"`+field+`"`) {
						return fmt.Errorf("stale field %s written: %s", field, actual)
					}
				}
				if !strings.Contains(parts[0], `"name" =`) {
					return fmt.Errorf("name patch missing: %s", actual)
				}
				return nil
			})
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
			require.NoError(t, err)
			defer db.Close()
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			name := "new name"
			ctx := service.WithOperatorResourcePatch(service.WithResourceManagerScope(context.Background(), 42, false), service.OperatorResourcePatch{Name: &name})
			mock.ExpectBegin()
			mock.ExpectExec("UPDATE").WillReturnError(sentinel)
			mock.ExpectRollback()
			staleOwner := int64(99)
			if entity == "account" {
				_, err = newAccountRepositoryWithSQL(client, db, nil).updateLockedAccount(ctx, client, &service.Account{ID: 12, ManagerUserID: &staleOwner, Name: "stale", Concurrency: 1, Priority: 1, Credentials: map[string]any{"api_key": "stale"}}, nil, nil, nil)
			} else {
				err = newGroupRepositoryWithSQL(client, db).Update(ctx, &service.Group{ID: 12, ManagerUserID: &staleOwner, Name: "stale", AllowOperatorContribution: true, RateMultiplier: 1})
			}
			require.ErrorIs(t, err, sentinel)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
