package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validJevRequestJSON() string {
	return `{
		"model": "jev-1.13",
		"state": "I was charged twice for my subscription.",
		"questions": {
			"refund": {
				"type": "noul",
				"instructions": "Is the customer asking for money back?"
			}
		}
	}`
}

func TestJevRequest_UnmarshalMarshal_RoundTrip(t *testing.T) {
	var req JevRequest
	require.NoError(t, json.Unmarshal([]byte(validJevRequestJSON()), &req))

	assert.Equal(t, "jev-1.13", req.Model)
	assert.JSONEq(t, `"I was charged twice for my subscription."`, string(req.State))
	require.Len(t, req.Questions, 1)
	refund := req.Questions["refund"]
	assert.Equal(t, "noul", refund.Type)
	assert.JSONEq(t, `"Is the customer asking for money back?"`, string(refund.Instructions))

	// Round-trip must not change the wire shape.
	known, err := json.Marshal(&req)
	require.NoError(t, err)

	var original, rebuilt map[string]any
	require.NoError(t, json.Unmarshal([]byte(validJevRequestJSON()), &original))
	require.NoError(t, json.Unmarshal(known, &rebuilt))
	assert.Equal(t, original, rebuilt)
}

func TestJevRequest_UnmarshalMarshal_StructuredState(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state string
	}{
		{
			name:  "object",
			state: `{"ticket":{"id":"T-1","messages":["charged twice"]},"policy":{"refund":true}}`,
		},
		{
			name:  "array",
			state: `[{"role":"user","content":"refund please"},{"role":"agent","content":"checking"}]`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"model":"jev-1.13","state":` + tc.state + `,"questions":{"refund":{"type":"noul","instructions":"Is a refund requested?"}}}`
			var req JevRequest
			require.NoError(t, json.Unmarshal([]byte(body), &req))
			require.NoError(t, req.Validate())

			forwarded, err := json.Marshal(&req)
			require.NoError(t, err)
			assert.JSONEq(t, body, string(forwarded), "model remapping must preserve the structured state")
		})
	}
}

func TestJevRequest_RawBodyCapture(t *testing.T) {
	body := validJevRequestJSON()
	var req JevRequest
	require.NoError(t, json.Unmarshal([]byte(body), &req))

	assert.Equal(t, "jev-1.13", req.ClientModel, "ClientModel records the model as the client sent it")
	require.NotEmpty(t, req.RawBody)
	assert.JSONEq(t, body, string(req.RawBody), "RawBody keeps the client's original request bytes")
}

func TestJevRequest_ExtraFieldsPreserved(t *testing.T) {
	body := `{
		"model": "jev-1.13",
		"state": "s",
		"questions": {
			"q1": {"type": "noul", "instructions": "i", "custom_hint": {"a": 1}}
		},
		"trace_id": "abc-123"
	}`
	var req JevRequest
	require.NoError(t, json.Unmarshal([]byte(body), &req))

	// Unknown top-level and question fields survive a re-marshal.
	out, err := json.Marshal(&req)
	require.NoError(t, err)
	assert.Contains(t, string(out), `"trace_id":"abc-123"`)
	assert.Contains(t, string(out), `"custom_hint":{"a":1}`)

	// Known fields are not duplicated into ExtraFields.
	assert.NotContains(t, string(out), `"model":"jev-1.13","model"`)
}

func TestJevRequest_Validate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		var req JevRequest
		require.NoError(t, json.Unmarshal([]byte(validJevRequestJSON()), &req))
		assert.NoError(t, req.Validate())
	})

	t.Run("missing model", func(t *testing.T) {
		req := &JevRequest{State: json.RawMessage(`"s"`), Questions: map[string]JevQuestion{"q": {Type: "noul"}}}
		assert.ErrorContains(t, req.Validate(), "model is required")
	})

	t.Run("missing state", func(t *testing.T) {
		req := &JevRequest{Model: "jev-1.13", Questions: map[string]JevQuestion{"q": {Type: "noul"}}}
		assert.ErrorContains(t, req.Validate(), "state is required")
	})

	t.Run("state null", func(t *testing.T) {
		req := &JevRequest{
			Model:     "jev-1.13",
			State:     json.RawMessage(`null`),
			Questions: map[string]JevQuestion{"q": {Type: "noul"}},
		}
		assert.ErrorContains(t, req.Validate(), "state is required")
	})

	t.Run("state empty string", func(t *testing.T) {
		req := &JevRequest{
			Model:     "jev-1.13",
			State:     json.RawMessage(`"  "`),
			Questions: map[string]JevQuestion{"q": {Type: "noul"}},
		}
		assert.ErrorContains(t, req.Validate(), "state is required")
	})

	t.Run("structured state object", func(t *testing.T) {
		req := &JevRequest{
			Model:     "jev-1.13",
			State:     json.RawMessage(`{"ticket":{"id":"T-1"},"order":{"amount":42},"policy":"refund"}`),
			Questions: map[string]JevQuestion{"q": {Type: "noul"}},
		}
		assert.NoError(t, req.Validate())
	})

	t.Run("missing questions", func(t *testing.T) {
		req := &JevRequest{Model: "jev-1.13", State: json.RawMessage(`"s"`)}
		assert.ErrorContains(t, req.Validate(), "questions is required")
	})

	t.Run("question without type", func(t *testing.T) {
		req := &JevRequest{
			Model:     "jev-1.13",
			State:     json.RawMessage(`"s"`),
			Questions: map[string]JevQuestion{"q": {Instructions: json.RawMessage(`"i"`)}},
		}
		assert.ErrorContains(t, req.Validate(), "questions.q.type is required")
	})

	t.Run("choice without criteria", func(t *testing.T) {
		req := &JevRequest{
			Model:     "jev-1.13",
			State:     json.RawMessage(`"s"`),
			Questions: map[string]JevQuestion{"q": {Type: "choice"}},
		}
		assert.ErrorContains(t, req.Validate(), "questions.q.criteria is required")
	})

	t.Run("choice with criteria", func(t *testing.T) {
		req := &JevRequest{
			Model: "jev-1.13",
			State: json.RawMessage(`"s"`),
			Questions: map[string]JevQuestion{
				"q": {Type: "choice", Criteria: json.RawMessage(`{"options":["a","b"]}`)},
			},
		}
		assert.NoError(t, req.Validate())
	})
}

func TestJevRequest_PromptText(t *testing.T) {
	req := &JevRequest{
		State: json.RawMessage(`"the state"`),
		Questions: map[string]JevQuestion{
			"beta": {Type: "noul", Instructions: json.RawMessage(`"beta instructions"`)},
			"alpha": {
				Type:     "choice",
				Criteria: json.RawMessage(`{"options":["refund","deny"]}`),
				ExtraFields: map[string]json.RawMessage{
					"hint": json.RawMessage(`"alpha hint"`),
				},
			},
		},
		ExtraFields: map[string]json.RawMessage{
			"locale": json.RawMessage(`"zh"`),
		},
	}
	// Everything forwarded to the upstream is covered in deterministic
	// order: state, then per-question (sorted names) instructions /
	// criteria / extra fields, then request-level extra fields. Empty
	// containers contribute nothing.
	assert.Equal(t,
		"the state\n{\"options\":[\"refund\",\"deny\"]}\nalpha hint\nbeta instructions\nzh",
		req.PromptText())

	// Deterministic across calls.
	assert.Equal(t, req.PromptText(), req.PromptText())
}

func TestJevRequest_PromptText_StructuredState(t *testing.T) {
	// A structured state contributes its compact JSON text so moderation
	// and token estimation see the full content.
	req := &JevRequest{
		State:     json.RawMessage(`{ "order": { "id": "A-1" }, "tags": ["refund"] }`),
		Questions: map[string]JevQuestion{},
	}
	assert.Equal(t, `{"order":{"id":"A-1"},"tags":["refund"]}`, req.PromptText())
}

func TestJevJSONToText(t *testing.T) {
	assert.Equal(t, "plain", JevJSONToText(json.RawMessage(`"plain"`)))
	assert.Equal(t, "{}", JevJSONToText(json.RawMessage(`{ }`)))
	assert.Equal(t, `[1,2]`, JevJSONToText(json.RawMessage(`[1, 2]`)))
	assert.Equal(t, "42", JevJSONToText(json.RawMessage(`42`)))
	assert.Empty(t, JevJSONToText(nil))
	assert.Empty(t, JevJSONToText(json.RawMessage(`null`)), "JSON null renders as empty")
}

func TestJevResponse_Parse(t *testing.T) {
	body := `{
		"id": "gen-dec-1789738314-X5e5eKGQdvR9rblyX250",
		"model": "typesafe/jev-1.13-20260917",
		"provider": "TypeSafe",
		"answers": {"refund": {"type": "noul", "noul": 0.98}},
		"usage": {"input_tokens": 275, "output_tokens": 20, "cost": 0.00003}
	}`
	var resp JevResponse
	require.NoError(t, json.Unmarshal([]byte(body), &resp))

	assert.Equal(t, "gen-dec-1789738314-X5e5eKGQdvR9rblyX250", resp.ID)
	assert.Equal(t, "typesafe/jev-1.13-20260917", resp.Model)
	assert.Equal(t, "TypeSafe", resp.Provider)
	require.Len(t, resp.Answers, 1)
	assert.JSONEq(t, `{"type": "noul", "noul": 0.98}`, string(resp.Answers["refund"]))
	assert.Equal(t, int64(275), resp.Usage.InputTokens)
	assert.Equal(t, int64(20), resp.Usage.OutputTokens)
	require.NotNil(t, resp.Usage.Cost)
	assert.InDelta(t, 0.00003, *resp.Usage.Cost, 1e-9)
	assert.Equal(t, int64(295), resp.Usage.TotalTokens())
}

func TestJevResponse_Parse_MissingOptionalFields(t *testing.T) {
	// id/model/provider are optional; the component client enforces the
	// required answers/usage fields on top of this decode.
	var resp JevResponse
	require.NoError(t, json.Unmarshal([]byte(`{"id":"gen-1","answers":{}}`), &resp))
	assert.Nil(t, resp.Usage)
	assert.NotNil(t, resp.Answers)
}
