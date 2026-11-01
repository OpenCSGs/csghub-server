package types

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestParseGuestIDHeaderRequiresUUIDv4(t *testing.T) {
	_, err := ParseGuestIDHeader("")
	require.Error(t, err)
	_, err = ParseGuestIDHeader("not-a-uuid")
	require.Error(t, err)
	_, err = ParseGuestIDHeader("00000000-0000-1000-8000-000000000000") // v1
	require.Error(t, err)
	id, err := ParseGuestIDHeader("550e8400-e29b-41d4-a716-446655440000")
	require.NoError(t, err)
	require.Equal(t, "550e8400-e29b-41d4-a716-446655440000", id)
}

func TestDeriveGuestActorIDIsStableAndOpaque(t *testing.T) {
	guestID := "550e8400-e29b-41d4-a716-446655440000"
	first := DeriveGuestActorID(guestID, 42)
	second := DeriveGuestActorID(guestID, 42)
	require.Equal(t, first, second)
	require.NotContains(t, first, guestID)
	require.True(t, len(first) > len("guest-"))
	require.NotEqual(t, first, DeriveGuestActorID(guestID, 43))
	require.NotEqual(t, first, DeriveGuestActorID(uuid.NewString(), 42))
}
