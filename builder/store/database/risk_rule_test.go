package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestRiskRuleStoreListsEnabledRules(t *testing.T) {
	ctx := context.Background()
	db, err := database.NewDB(ctx, database.DBConfig{Dialect: database.DialectSQLite, DSN: "file:risk_store?mode=memory&cache=shared", DisableAuditHook: true})
	require.NoError(t, err)
	defer db.Close()
	_, err = db.Core.ExecContext(ctx, `CREATE TABLE risk_email_domains (domain TEXT, enabled BOOLEAN);
CREATE TABLE risk_phone_prefixes (phone_area TEXT, prefix TEXT, enabled BOOLEAN);
INSERT INTO risk_email_domains VALUES ('listed.test', TRUE), ('disabled.test', FALSE);
INSERT INTO risk_phone_prefixes VALUES ('+86', '170', TRUE), ('+1', '555', TRUE), ('+86', '171', FALSE)`)
	require.NoError(t, err)
	store := database.NewRiskRuleStoreWithDB(db)
	domains, err := store.ListEmailDomains(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"listed.test"}, domains)
	prefixes, err := store.ListPhonePrefixes(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, []types.RiskPhonePrefix{{PhoneArea: "+86", Prefix: "170"}, {PhoneArea: "+1", Prefix: "555"}}, prefixes)
	_, err = db.Core.ExecContext(ctx, "DROP TABLE risk_email_domains")
	require.NoError(t, err)
	_, err = store.ListEmailDomains(ctx)
	require.Error(t, err)
	prefixes, err = store.ListPhonePrefixes(ctx)
	require.NoError(t, err)
	require.Len(t, prefixes, 2)
	_, err = db.Core.ExecContext(ctx, "DROP TABLE risk_phone_prefixes")
	require.NoError(t, err)
	_, err = store.ListPhonePrefixes(ctx)
	require.Error(t, err)
}
