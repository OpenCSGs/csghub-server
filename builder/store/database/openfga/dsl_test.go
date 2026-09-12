package openfga

import (
	"strings"
	"testing"

	parser "github.com/openfga/language/pkg/go/transformer"
	"github.com/stretchr/testify/require"
)

func TestAuthorizationModelDSLVersions(t *testing.T) {
	tests := []struct {
		name string
		dsl  string
	}{
		{name: "version 1.0", dsl: AuthorizationModelVer1_0},
		{name: "version 1.1", dsl: AuthorizationModelVer1_1},
		{name: "version 1.2", dsl: AuthorizationModelVer1_2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, err := parser.TransformDSLToProto(tt.dsl)
			require.NoError(t, err)
			require.NotNil(t, model)
			require.Equal(t, "1.1", model.GetSchemaVersion())
		})
	}

	require.NotContains(t, AuthorizationModelVer1_0, "define member_from_child")
	require.True(t, strings.Contains(AuthorizationModelVer1_1, "define member_from_child"))
	require.True(t, strings.Contains(AuthorizationModelVer1_2, "define member_from_child"))
	require.NotContains(t, AuthorizationModelVer1_1, "type knowledge_base")
	require.Contains(t, AuthorizationModelVer1_2, "type knowledge_base")
}
