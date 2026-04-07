package common

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMarshalJSONWithoutHTMLEscape_preservesAmpersandInURLs(t *testing.T) {
	type payload struct {
		URL string `json:"url"`
	}
	u := "https://example.com/obj.png?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Signature=abc"

	out, err := MarshalJSONWithoutHTMLEscape(payload{URL: u})
	require.NoError(t, err)
	require.Contains(t, string(out), "&X-Amz-Signature")
	require.NotContains(t, string(out), `\u0026`)

	var std []byte
	std, err = json.Marshal(payload{URL: u})
	require.NoError(t, err)
	require.Contains(t, string(std), `\u0026`)
}

func TestMarshalJSONWithoutHTMLEscape_roundTrip(t *testing.T) {
	type payload struct {
		A string `json:"a"`
	}
	in := payload{A: "x&y<z>"}
	out, err := MarshalJSONWithoutHTMLEscape(in)
	require.NoError(t, err)
	var got payload
	require.NoError(t, json.Unmarshal(out, &got))
	require.Equal(t, in, got)
}
