package rebac

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClassifyError(t *testing.T) {
	tests := []struct {
		err      error
		expected ErrorClass
	}{
		{err: nil, expected: ErrorClassNone},
		{err: ErrDenied, expected: ErrorClassDenied},
		{err: ErrInvalidRequest, expected: ErrorClassInvalid},
		{err: ErrUnsupportedRelation, expected: ErrorClassUnsupported},
		{err: ErrProviderTimeout, expected: ErrorClassTimeout},
		{err: ErrProviderUnavailable, expected: ErrorClassUnavailable},
		{err: context.Canceled, expected: ErrorClassCanceled},
		{err: fmt.Errorf("other"), expected: ErrorClassUnknown},
	}
	for _, test := range tests {
		require.Equal(t, test.expected, ClassifyError(test.err))
	}
}
