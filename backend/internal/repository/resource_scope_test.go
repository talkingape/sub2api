package repository

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestResourceManagerQueriesScopeCountAndRows(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		group, scope, public bool
	}{
		{"account scoped", false, true, false}, {"account admin", false, false, false},
		{"group scoped", true, true, false}, {"group public", true, true, true}, {"group admin", true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			matcher := sqlmock.QueryMatcherFunc(func(expected, actual string) error {
				if !strings.Contains(actual, expected) {
					return fmt.Errorf("missing %s in %s", expected, actual)
				}
				// The SELECT projection always contains manager_user_id; inspect only WHERE.
				where := actual[strings.Index(actual, " WHERE ")+1:]
				hasOwner := strings.Contains(where, `"manager_user_id" =`)
				if hasOwner != tc.scope {
					return fmt.Errorf("unexpected ownership predicate: %s", actual)
				}
				hasPublic := strings.Contains(where, `"allow_operator_contribution"`)
				if hasPublic != tc.public {
					return fmt.Errorf("unexpected public predicate: %s", actual)
				}
				return nil
			})
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(matcher))
			require.NoError(t, err)
			defer db.Close()
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			ctx := context.Background()
			if tc.scope {
				ctx = service.WithResourceManagerScope(ctx, 42, tc.public)
			}
			mock.ExpectQuery("COUNT").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			table := "accounts"
			if tc.group {
				table = "groups"
			}
			mock.ExpectQuery(`FROM "` + table + `"`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			params := pagination.PaginationParams{Page: 1, PageSize: 10}
			if tc.group {
				_, _, err = newGroupRepositoryWithSQL(client, db).ListWithFilters(ctx, params, "", "", "", nil)
			} else {
				_, _, err = newAccountRepositoryWithSQL(client, db, nil).ListWithFilters(ctx, params, "", "", "", "", 0, "")
			}
			require.NoError(t, err)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
