package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/testutil"
)

func TestQueryBool(t *testing.T) {
	for _, tc := range []struct {
		name          string
		rawParam      string
		addParam      bool
		want          bool
		wantErrLogged bool
	}{
		{name: "absent param returns false", addParam: false, want: false, wantErrLogged: false},
		{name: "empty param returns false", addParam: true, rawParam: "", want: false, wantErrLogged: false},
		{name: "true returns true", addParam: true, rawParam: "true", want: true, wantErrLogged: false},
		{name: "false returns false", addParam: true, rawParam: "false", want: false, wantErrLogged: false},
		{name: "one returns true", addParam: true, rawParam: "1", want: true, wantErrLogged: false},
		{name: "invalid param returns false and logs error", addParam: true, rawParam: "abc", want: false, wantErrLogged: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs := captureSlog(t)
			tester := testutil.NewGinTester()
			if tc.addParam {
				tester.WithQuery("need_multi_sync", tc.rawParam)
			}
			got := queryBool(tester.Gctx(), "need_multi_sync")
			require.Equal(t, tc.want, got)
			if tc.wantErrLogged {
				require.Contains(t, logs.String(), "bad need_multi_sync params")
			} else {
				require.NotContains(t, logs.String(), "bad need_multi_sync params")
			}
		})
	}
}
