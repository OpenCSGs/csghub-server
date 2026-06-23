# AIGateway Responses API Local Test Guide

This guide verifies OpenAI-compatible `POST /v1/responses` in local AIGateway.

## 1. Start AIGateway

From the repo root:

```bash
go run -tags "saas license_issuer" cmd/csghub-server/main.go aigateway launch -l debug --config .vscode/config.toml
```

Common local base URLs:

```bash
# Default local AIGateway
export CSGHUB_AIGATEWAY_BASE_URL_LOCAL="http://localhost:8094/v1"

# If using VSCode launch config "aigateway-2"
export CSGHUB_AIGATEWAY_BASE_URL_LOCAL="http://localhost:8099/v1"

export CSGHUB_API_KEY_LOCAL="<your api key>"
```

The route registered by this feature is:

```text
POST /v1/responses
```

If `CSGHUB_AIGATEWAY_BASE_URL_LOCAL` already includes `/v1`, call:

```text
$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses
```

## 2. Run Unit Tests

```bash
go test ./aigateway/handler ./aigateway/types
go test ./aigateway/...
```

## 3. Basic Non-streaming Response

```bash
curl -s "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "minimax-m2.7",
    "input": "Tell me a joke."
  }' | jq .
```

Expected:

- `object` is `response`.
- `status` is `completed`.
- `model` is the public model id.
- `output` contains a message item.
- `output_text` contains the final text when mapped through chat adapter.
- `usage` is present if the upstream returned usage.

## 4. Basic Streaming Response

```bash
curl -s -N "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "minimax-m2.7",
    "input": "Tell me a joke.",
    "stream": true
  }'
```

Expected event sequence includes:

```text
event: response.created
event: response.in_progress
event: response.output_item.added
event: response.content_part.added
event: response.output_text.delta
event: response.output_text.done
event: response.content_part.done
event: response.output_item.done
event: response.completed
data: [DONE]
```

Check final event and usage:

```bash
curl -s -N "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "minimax-m2.7",
    "input": "Tell me a short joke.",
    "stream": true
  }' | tee /tmp/responses-stream.log

grep 'response.completed' /tmp/responses-stream.log
grep 'data: \[DONE\]' /tmp/responses-stream.log
grep '"usage"' /tmp/responses-stream.log
```

Notes:

- Adapter mode requests upstream Chat Completions with `stream_options.include_usage=true`.
- `usage` appears in `response.completed` only if the upstream emits a usage chunk.

## 5. Instructions And Structured Output

```bash
curl -s "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "minimax-m2.7",
    "instructions": "You are a strict JSON generator. Return only valid JSON.",
    "input": "Extract name and city from: Alice lives in Tokyo.",
    "text": {
      "format": {
        "type": "json_schema",
        "json_schema": {
          "name": "person_location",
          "schema": {
            "type": "object",
            "properties": {
              "name": {"type": "string"},
              "city": {"type": "string"}
            },
            "required": ["name", "city"],
            "additionalProperties": false
          }
        }
      }
    }
  }' | jq .
```

Adapter mapping:

- `instructions` becomes a leading system message.
- `text.format` maps to Chat Completions `response_format`.

## 6. Multi-message Input

```bash
curl -s "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "minimax-m2.7",
    "input": [
      {
        "role": "user",
        "content": "My name is Jun."
      },
      {
        "role": "assistant",
        "content": "Nice to meet you, Jun."
      },
      {
        "role": "user",
        "content": "What is my name?"
      }
    ]
  }' | jq .
```

Expected:

- Adapter mode maps the input array to Chat Completions `messages`.
- The model should answer with the remembered name from the request body.

## 7. Text Content Parts

```bash
curl -s "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "minimax-m2.7",
    "input": [
      {
        "role": "user",
        "content": [
          {
            "type": "input_text",
            "text": "Summarize this in one sentence."
          },
          {
            "type": "input_text",
            "text": "AIGateway maps Responses API requests to native Responses or Chat Completions upstreams."
          }
        ]
      }
    ]
  }' | jq .
```

Expected:

- `input_text` parts are mapped to chat-compatible text parts.

## 8. Streaming Function Tool

```bash
curl -s -N "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-v4-flash",
    "input": "Use the tool to get weather in Tokyo.",
    "stream": true,
    "tools": [
      {
        "type": "function",
        "name": "get_weather",
        "description": "Get current weather by city.",
        "parameters": {
          "type": "object",
          "properties": {
            "city": {"type": "string"},
            "unit": {"type": "string", "enum": ["celsius", "fahrenheit"]}
          },
          "required": ["city"]
        }
      }
    ],
    "tool_choice": "auto"
  }'
```

Expected tool events:

```text
event: response.output_item.added
event: response.function_call_arguments.delta
event: response.function_call_arguments.done
event: response.output_item.done
event: response.completed
data: [DONE]
```

## 9. Multiple Tools

```bash
curl -s -N "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-v4-flash",
    "input": "Find weather in Tokyo and convert 100 USD to JPY.",
    "stream": true,
    "parallel_tool_calls": true,
    "tools": [
      {
        "type": "function",
        "name": "get_weather",
        "description": "Get current weather by city.",
        "parameters": {
          "type": "object",
          "properties": {
            "city": {"type": "string"}
          },
          "required": ["city"]
        }
      },
      {
        "type": "function",
        "name": "convert_currency",
        "description": "Convert money between currencies.",
        "parameters": {
          "type": "object",
          "properties": {
            "amount": {"type": "number"},
            "from": {"type": "string"},
            "to": {"type": "string"}
          },
          "required": ["amount", "from", "to"]
        }
      }
    ]
  }'
```

Expected:

- One or more function call output items.
- `parallel_tool_calls` is forwarded to Chat Completions in adapter mode.

## 10. Force A Specific Tool

```bash
curl -s "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-v4-flash",
    "input": "Tokyo",
    "tools": [
      {
        "type": "function",
        "name": "get_weather",
        "description": "Get current weather by city.",
        "parameters": {
          "type": "object",
          "properties": {
            "city": {"type": "string"}
          },
          "required": ["city"]
        }
      }
    ],
    "tool_choice": {
      "type": "function",
      "function": {
        "name": "get_weather"
      }
    }
  }' | jq .
```

Expected:

- Response contains a `function_call` output item if the upstream honors forced tool choice.

## 11. Adapter Unsupported Feature: `store:true`

```bash
curl -s "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "minimax-m2.7",
    "input": "hello",
    "store": true
  }' | jq .
```

Expected:

```json
{
  "error": {
    "type": "invalid_request_error",
    "code": "unsupported_feature"
  }
}
```

## 12. Adapter Unsupported Feature: `previous_response_id`

```bash
curl -s "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "minimax-m2.7",
    "previous_response_id": "resp_agw_adapter_fake",
    "input": "Continue."
  }' | jq .
```

Expected:

```json
{
  "error": {
    "type": "invalid_request_error",
    "code": "unsupported_feature"
  }
}
```

## 13. Adapter Unsupported Feature: `reasoning`

```bash
curl -s "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "minimax-m2.7",
    "input": "Think deeply and answer.",
    "reasoning": {
      "effort": "high"
    }
  }' | jq .
```

Expected:

```json
{
  "error": {
    "type": "invalid_request_error",
    "code": "unsupported_feature"
  }
}
```

## 14. Native Responses Mode

Native mode is selected when the configured upstream endpoint path ends with:

```text
/responses
```

Adapter mode is selected when the configured upstream endpoint path ends with:

```text
/chat/completions
```

Native mode should:

- Forward unknown request fields.
- Rewrite public model id to upstream model id.
- Wrap native upstream response ids as `resp_agw_v1...`.
- Unwrap `previous_response_id` and route to the original upstream id.
- Not emulate gateway-owned conversation state.

If the upstream URL does not end with `/responses` or `/chat/completions`, `/v1/responses` returns `unsupported_feature`.

Set a model id that is backed by an upstream URL ending in `/responses`:

```bash
export CSGHUB_NATIVE_RESPONSES_MODEL="<native responses model id>"
```

Before testing, confirm the model's upstream LLM config uses a native Responses endpoint, for example:

```text
https://api.openai.com/v1/responses
https://<azure-resource>.openai.azure.com/openai/deployments/<deployment>/responses?api-version=<api-version>
```

If the same model is configured with `/chat/completions`, these requests will use adapter mode instead.

### 14.1 Native Non-streaming Passthrough

```bash
curl -s "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "'"$CSGHUB_NATIVE_RESPONSES_MODEL"'",
    "input": "Say hello in one short sentence.",
    "metadata": {
      "test_case": "native_non_stream"
    }
  }' | tee /tmp/native-responses.json | jq .
```

Expected:

- `object` is `response`.
- `model` is the public AIGateway model id or the upstream model id returned by the native provider.
- `id` starts with `resp_agw_v1.` because AIGateway wraps native upstream response ids.
- Native fields returned by the upstream are preserved.
- `usage` is recorded when the upstream includes a Responses `usage` block.

Extract the wrapped response id for follow-up tests:

```bash
export CSGHUB_NATIVE_RESPONSE_ID="$(jq -r '.id' /tmp/native-responses.json)"
echo "$CSGHUB_NATIVE_RESPONSE_ID"
```

### 14.2 Native Streaming Passthrough

```bash
curl -s -N "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "'"$CSGHUB_NATIVE_RESPONSES_MODEL"'",
    "input": "Tell me a one sentence joke.",
    "stream": true
  }' | tee /tmp/native-responses-stream.log
```

Expected:

- Native Responses SSE event names are preserved.
- Response ids in event payloads are wrapped as `resp_agw_v1...`.
- The stream includes the upstream terminal event, normally `response.completed`.
- If the upstream emits `event: error`, AIGateway relays that event as-is and does not synthesize `response.completed`.

Quick checks:

```bash
grep 'event: response.created' /tmp/native-responses-stream.log
grep 'event: response.completed' /tmp/native-responses-stream.log
grep 'resp_agw_v1' /tmp/native-responses-stream.log
```

Check whether the native stream includes usage:

```bash
grep '"usage"' /tmp/native-responses-stream.log
```

Expected:

- If the upstream emits usage in stream mode, AIGateway should preserve it in the streamed payload.
- If usage is absent, confirm the upstream supports streamed Responses usage before treating it as an AIGateway bug.

### 14.3 Native `previous_response_id` State Continuity

Native `previous_response_id` uses upstream-owned response state. AIGateway only unwraps the gateway response id and routes the follow-up request to the original upstream id.

```bash
curl -s "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "'"$CSGHUB_NATIVE_RESPONSES_MODEL"'",
    "previous_response_id": "'"$CSGHUB_NATIVE_RESPONSE_ID"'",
    "input": "Continue from the previous response in one short sentence."
  }' | jq .
```

Expected:

- Request succeeds only when the wrapped id belongs to the same namespace and original upstream route.
- AIGateway unwraps `previous_response_id` before proxying to the native upstream.
- AIGateway does not store or replay conversation history.
- If the original upstream route is unavailable, the response error code is `response_route_unavailable`.

Run a stricter continuity check with a fact from the first response:

```bash
curl -s "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "'"$CSGHUB_NATIVE_RESPONSES_MODEL"'",
    "input": "Remember this test code: native-route-42. Reply with only ok."
  }' | tee /tmp/native-responses-state-1.json | jq .

export CSGHUB_NATIVE_STATE_ID="$(jq -r '.id' /tmp/native-responses-state-1.json)"

curl -s "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "'"$CSGHUB_NATIVE_RESPONSES_MODEL"'",
    "previous_response_id": "'"$CSGHUB_NATIVE_STATE_ID"'",
    "input": "What test code did I ask you to remember? Reply with only the code."
  }' | tee /tmp/native-responses-state-2.json | jq .
```

Expected:

- The second response should reference `native-route-42` if the upstream provider supports `previous_response_id`.
- AIGateway should not require the client to resend the first request body.
- The second response should expose a new wrapped `id`, not the raw upstream id.

### 14.4 Native Unknown Field Passthrough

Use a harmless unknown field to confirm native passthrough preserves fields that AIGateway does not understand:

```bash
curl -s "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "'"$CSGHUB_NATIVE_RESPONSES_MODEL"'",
    "input": "Return the word ok.",
    "x_aigateway_passthrough_probe": {
      "enabled": true
    }
  }' | jq .
```

Expected:

- AIGateway does not reject the unknown field.
- The upstream may ignore or reject the field according to its own validation rules.
- If the upstream rejects it, the error should come from the upstream, not from AIGateway request DTO validation.

### 14.5 Native Structured Output

This verifies that `text.format` is forwarded to the native upstream without adapter-side downgrade or validation.

```bash
curl -s "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "'"$CSGHUB_NATIVE_RESPONSES_MODEL"'",
    "instructions": "Return only valid JSON.",
    "input": "Extract name and city from: Alice lives in Tokyo.",
    "text": {
      "format": {
        "type": "json_schema",
        "json_schema": {
          "name": "person_location",
          "schema": {
            "type": "object",
            "properties": {
              "name": {"type": "string"},
              "city": {"type": "string"}
            },
            "required": ["name", "city"],
            "additionalProperties": false
          }
        }
      }
    },
    "metadata": {
      "test_case": "native_json_schema"
    }
  }' | tee /tmp/native-responses-json-schema.json | jq .
```

Expected:

- AIGateway forwards `text.format` unchanged.
- If the native upstream supports `json_schema`, response output should contain JSON matching the schema.
- If the native upstream rejects `json_schema`, the error should come from the upstream; AIGateway should not rewrite it to `json_object` or prompt text.

Quick output-text check:

```bash
jq -r '.output_text // empty' /tmp/native-responses-json-schema.json
```

### 14.6 Native Function Tool Call

This verifies native tool payload passthrough. The request uses Responses-style function tools, not Chat Completions nested `function` format.

```bash
curl -s "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "'"$CSGHUB_NATIVE_RESPONSES_MODEL"'",
    "input": "What is the weather in Tokyo?",
    "tools": [{
      "type": "function",
      "name": "get_weather",
      "description": "Get current weather by city.",
      "parameters": {
        "type": "object",
        "properties": {
          "city": {"type": "string"}
        },
        "required": ["city"],
        "additionalProperties": false
      }
    }],
    "tool_choice": {
      "type": "function",
      "name": "get_weather"
    },
    "metadata": {
      "test_case": "native_function_tool"
    }
  }' | tee /tmp/native-responses-tool.json | jq .
```

Expected:

- AIGateway forwards the tool definition unchanged.
- If the native upstream honors `tool_choice`, `output` should contain a `function_call` item.
- Response IDs inside tool-related output items remain wrapped if they refer to a response id.

Quick function-call check:

```bash
jq '.output[]? | select(.type == "function_call")' /tmp/native-responses-tool.json
```

### 14.7 Native Hosted Tools Passthrough

This verifies native-provider hosted tools are forwarded unchanged. These tools are not supported by adapter mode; they require an upstream URL ending in `/responses`.

Python SDK example:

```python
from openai import OpenAI

client = OpenAI(
    api_key=os.environ["CSGHUB_API_KEY_LOCAL"],
    base_url=os.environ["CSGHUB_AIGATEWAY_BASE_URL_LOCAL"],
)

response = client.responses.create(
    model="qwen3.7-max",
    input="杭州天气",
    tools=[
        {"type": "web_search"},
        {"type": "web_extractor"},
        {"type": "code_interpreter"},
    ],
    extra_body={"enable_thinking": True},
)

print(response)
```

Equivalent curl:

```bash
curl -s "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "'"${CSGHUB_NATIVE_RESPONSES_MODEL:-qwen3.7-plus}"'",
    "input": "杭州天气",
    "tools": [
      {"type": "web_search"},
      {"type": "web_extractor"},
      {"type": "code_interpreter"}
    ],
    "enable_thinking": true,
    "metadata": {
      "test_case": "native_hosted_tools"
    }
  }' | tee /tmp/native-responses-hosted-tools.json | jq .
```

Expected:

- AIGateway forwards hosted tool definitions unchanged.
- AIGateway preserves unknown native fields such as `enable_thinking`.
- If the native upstream supports these tools, `output` may contain web/tool/code related items plus final assistant text.
- If the native upstream does not support one tool, the error should come from upstream capability validation.

Quick checks:

```bash
jq -r '.id' /tmp/native-responses-hosted-tools.json
jq '.output[]?.type' /tmp/native-responses-hosted-tools.json
jq -r '.output_text // empty' /tmp/native-responses-hosted-tools.json
```

### 14.8 Native Streaming Hosted Tools

```bash
curl -s -N "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "'"${CSGHUB_NATIVE_RESPONSES_MODEL:-qwen3.7-plus}"'",
    "input": "搜索并总结杭州今天的天气，最后给出一句出行建议。",
    "stream": true,
    "tools": [
      {"type": "web_search"},
      {"type": "web_extractor"},
      {"type": "code_interpreter"}
    ],
    "enable_thinking": true
  }' | tee /tmp/native-responses-hosted-tools-stream.log
```

Expected:

- Native SSE events are relayed with provider event names preserved.
- Response ids in event payloads are wrapped as `resp_agw_v1...`.
- Hosted tool events, reasoning events, and text events are not converted to chat events.
- The stream should end with upstream `response.completed` unless the upstream emits `event: error`.

Quick checks:

```bash
grep 'event: response.' /tmp/native-responses-hosted-tools-stream.log | head
grep 'resp_agw_v1' /tmp/native-responses-hosted-tools-stream.log
grep 'response.completed' /tmp/native-responses-hosted-tools-stream.log
grep '"usage"' /tmp/native-responses-hosted-tools-stream.log
```

### 14.9 Native Streaming Function Tool Call

```bash
curl -s -N "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "'"$CSGHUB_NATIVE_RESPONSES_MODEL"'",
    "input": "What is the weather in Tokyo?",
    "stream": true,
    "tools": [{
      "type": "function",
      "name": "get_weather",
      "description": "Get current weather by city.",
      "parameters": {
        "type": "object",
        "properties": {
          "city": {"type": "string"}
        },
        "required": ["city"]
      }
    }],
    "tool_choice": {
      "type": "function",
      "name": "get_weather"
    }
  }' | tee /tmp/native-responses-tool-stream.log
```

Expected event sequence depends on the native provider, but normally includes:

```text
event: response.output_item.added
event: response.function_call_arguments.delta
event: response.function_call_arguments.done
event: response.output_item.done
event: response.completed
```

Quick checks:

```bash
grep 'response.function_call_arguments.delta' /tmp/native-responses-tool-stream.log
grep 'response.completed' /tmp/native-responses-tool-stream.log
grep 'resp_agw_v1' /tmp/native-responses-tool-stream.log
```

### 14.10 Native Response ID Owner Check

Use a deliberately invalid wrapped id to verify AIGateway rejects IDs it cannot unwrap.

```bash
curl -s "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "'"$CSGHUB_NATIVE_RESPONSES_MODEL"'",
    "previous_response_id": "resp_agw_v1.invalid",
    "input": "Continue."
  }' | jq .
```

Expected:

```json
{
  "error": {
    "type": "invalid_request_error",
    "code": "invalid_response_id"
  }
}
```

If testing with another namespace's real wrapped id, expected code is `response_id_forbidden`.

### 14.11 Native Raw Upstream Response ID Rejection

AIGateway clients must use wrapped gateway response ids. Raw upstream ids should not be accepted as `previous_response_id`.

```bash
curl -s "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "'"$CSGHUB_NATIVE_RESPONSES_MODEL"'",
    "previous_response_id": "resp_raw_upstream_id_for_test",
    "input": "Continue."
  }' | jq .
```

Expected:

```json
{
  "error": {
    "type": "invalid_request_error",
    "code": "invalid_response_id"
  }
}
```

### 14.12 Native Multimodal Input Passthrough

Use this only with a native Responses model that supports image input.

```bash
curl -s "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "'"$CSGHUB_NATIVE_RESPONSES_MODEL"'",
    "input": [{
      "role": "user",
      "content": [
        {
          "type": "input_text",
          "text": "Describe this image in one short sentence."
        },
        {
          "type": "input_image",
          "image_url": "https://upload.wikimedia.org/wikipedia/commons/thumb/3/3f/Fronalpstock_big.jpg/640px-Fronalpstock_big.jpg"
        }
      ]
    }],
    "metadata": {
      "test_case": "native_multimodal"
    }
  }' | jq .
```

Expected:

- AIGateway forwards the `input_image` part unchanged.
- If the upstream supports image input, the response should describe the image.
- If the upstream rejects image input, the error should come from upstream capability validation.

### 14.13 Native Reasoning And Include Passthrough

Use this only with a native Responses model that supports reasoning controls or include fields.

```bash
curl -s "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "'"$CSGHUB_NATIVE_RESPONSES_MODEL"'",
    "input": "Solve 19 * 23 and give only the final number.",
    "reasoning": {
      "effort": "low",
      "summary": "auto"
    },
    "include": [
      "reasoning.encrypted_content"
    ],
    "metadata": {
      "test_case": "native_reasoning_include"
    }
  }' | jq .
```

Expected:

- AIGateway forwards `reasoning` and `include` unchanged.
- AIGateway should not inspect, store, or transform provider reasoning payloads.
- Unsupported `reasoning` or `include` values should be rejected by the native upstream, not by the gateway.

### 14.14 Native Store Flag Passthrough

Native mode lets the upstream provider decide whether `store` is supported.

```bash
curl -s "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "'"$CSGHUB_NATIVE_RESPONSES_MODEL"'",
    "input": "Say stored if this request is accepted.",
    "store": true,
    "metadata": {
      "test_case": "native_store_true"
    }
  }' | jq .
```

Expected:

- AIGateway forwards `store: true` in native mode.
- If the upstream supports stored Responses, the request succeeds.
- If the upstream does not support stored Responses, the upstream error is returned.
- AIGateway does not create gateway-owned stored response state.

### 14.15 Native Stream Error Passthrough

This is easiest to verify against a native upstream or mock provider that can emit an SSE `event: error`.

Expected AIGateway behavior:

- Relay the upstream `event: error` frame as-is.
- Do not synthesize `response.completed` after the error.
- Do not rewrite the error payload into a gateway-specific shape.

If a local mock upstream is available, configure the model upstream URL to a mock `/v1/responses` endpoint that emits:

```text
event: error
data: {"error":{"message":"mock stream error","type":"api_error","code":"mock_error"}}
```

Then run:

```bash
curl -s -N "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "'"$CSGHUB_NATIVE_RESPONSES_MODEL"'",
    "input": "trigger stream error",
    "stream": true
  }' | tee /tmp/native-responses-error-stream.log
```

Quick checks:

```bash
grep 'event: error' /tmp/native-responses-error-stream.log
! grep 'event: response.completed' /tmp/native-responses-error-stream.log
```

### 14.16 Native Compression Check

Native Responses disables upstream compression because AIGateway must inspect JSON/SSE payloads for response ID rewriting and usage capture.

```bash
curl -i "$CSGHUB_AIGATEWAY_BASE_URL_LOCAL/responses" \
  -H "Authorization: Bearer $CSGHUB_API_KEY_LOCAL" \
  -H "Content-Type: application/json" \
  -H "Accept-Encoding: gzip" \
  -d '{
    "model": "'"$CSGHUB_NATIVE_RESPONSES_MODEL"'",
    "input": "Say ok."
  }'
```

Expected:

- Response body should be readable JSON in the terminal.
- `Content-Encoding: gzip` should not be present unless an intermediate proxy recompresses after AIGateway.
- If gzip still appears, inspect the path after AIGateway, such as Istio/Envoy or an external ingress.

## 15. Troubleshooting

### Missing `response.completed`

Make sure the local AIGateway process is running the latest branch. Adapter stream finalization emits `response.completed` and `data: [DONE]` after the chat proxy finishes, even if the upstream does not send `[DONE]`.

### Missing `usage`

Usage is included in `response.completed.response.usage` only if the upstream Chat Completions stream emits a usage chunk. The adapter sends:

```json
{"stream_options":{"include_usage":true}}
```

Some upstreams may ignore this option.

### Tool schema error: missing `function`

Responses function tools use:

```json
{
  "type": "function",
  "name": "get_weather",
  "parameters": {}
}
```

The adapter converts this to Chat Completions format:

```json
{
  "type": "function",
  "function": {
    "name": "get_weather",
    "parameters": {}
  }
}
```

If upstream still rejects it, inspect the proxied provider's expected tool format.

### Reasoning text appears as `<think>...</think>`

The generic adapter does not strip provider-specific reasoning tags. If a provider emits reasoning inside normal Chat Completions `content`, AIGateway forwards it as `output_text`.
