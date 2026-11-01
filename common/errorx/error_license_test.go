package errorx

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCustomerLicenseNotFound(t *testing.T) {
	err := CustomerLicenseNotFound(sql.ErrNoRows, Ctx().Set("product", "CSGLite"))
	require.ErrorIs(t, err, ErrCustomerLicenseNotFound)
	require.ErrorIs(t, err, sql.ErrNoRows)
	custom, ok := err.(CustomError)
	require.True(t, ok)
	require.Equal(t, "LICENSE-ERR-5", custom.Code())
	require.Equal(t, "CSGLite", custom.Context()["product"])
}
