# LLMLog Training V1

## Goal

Record training-usable LLM chat data at the AIGateway layer instead of relying on agent-side conversation history.

The main gap in existing session history is that it is user-centric and misses tool schemas and structured model I/O. That makes it weak as direct model training data. V1 fixes that by capturing normalized chat-completion training samples in AIGateway and asynchronously persisting them through a dedicated `llmlog` service.

## Final V1 Decisions

- Scope only `/v1/chat/completions`
- Focus only on training data, not full observability replay
- Store logs as JSONL in object storage
- Use one training record per successful request
- Keep only `request_id`
- `request_id` is read from Gin `trace_id`
- Keep raw content, no desensitization
- New service name is `llmlog`
- Service should be extensible for future query support

## Training Record Shape

Each successful chat request is normalized into one JSONL line with these top-level fields:

- `record_type`
- `request_id`
- `event_time`
- `sample_type`
- `model_id`
- `user_uuid`
- `tools`
- `messages`
- `usage`
- `metadata`

`messages` supports:

- `system`
- `user`
- `assistant`
- `tool_call`
- `tool_response`

`tools` keeps the full tool schema so the sample is usable for training and tool-use supervision.

There is no top-level `output` field in the shipped V1 schema. The model response is merged into `messages` so the final record is directly usable for SFT pipelines.

Normalization rules implemented in V1:

- final assistant output is appended into `messages`
- streamed `tool_calls` are merged by `index` before normalization
- `tool_call.content` is stored as JSON text with `arguments` as a compact JSON string
- `tool_response.content` is compacted if it is valid JSON
- `finish_reason` is stored in `metadata.finish_reason`
- multipart request content is treated as text-only in V1; only `type == "text"` parts are retained in archived message content

## Architecture

### AIGateway

AIGateway captures the normalized training sample during `/v1/chat/completions` handling.

- Read `trace_id` from Gin context and store it as `request_id`
- Normalize request messages and tools
- Capture final assistant output and merge it into `messages`
- For non-streaming responses, normalize the final completion body into message form
- For streaming responses, aggregate stream chunks, merge `tool_calls` by `index`, then normalize into message form
- Publish one asynchronous MQ event after the request completes
- Keep handler-to-builder coupling out of the handler by routing llmlog publishing through a component-level publisher interface

V1 only publishes successful training samples. It does not publish failed requests as training data.

### LLMLog

`llmlog` is a dedicated consumer-side service.

- Subscribe to the AIGateway training subject
- Batch records in memory
- Serialize records into JSONL
- Flush to object storage as part files

Current object key pattern:

`{prefix}/dt=YYYY-MM-DD/hour=HH/part-{unixnano}-{uuid}.jsonl`

Content type used for uploads:

`application/x-ndjson`

## Implemented Files

### Shared Types

- `common/types/llm_log.go`

Added:

- `LLMLogUsage`
- `LLMTrainingMessage`
- `LLMTrainingLogRecord`

### AIGateway Capture

- `aigateway/handler/training_capture.go`
- `aigateway/handler/training_capture_test.go`

Added a training capture helper that:

- normalizes request messages
- preserves tools
- appends final completion content into `messages`
- merges streamed tool-call deltas into finalized tool calls
- compacts tool-call arguments and tool-response JSON strings
- builds the final `LLMTrainingLogRecord`

### AIGateway Response Wrappers

- `aigateway/handler/response_writer_wrapper.go`
- `aigateway/handler/response_writer_wrapper_non_stream.go`

Extended wrappers to optionally feed a `trainingRecorder` without breaking existing callers.

- stream wrapper aggregates chunks into the recorder
- non-stream wrapper passes the final completion to the recorder

### AIGateway Chat Handler

- `aigateway/handler/openai.go`
- `aigateway/component/llmlog_publisher.go`

Changes:

- created a training capture inside chat handling
- passed the recorder into the response wrapper
- split token usage recording and llmlog publishing into separate goroutines
- published training logs through `component.LLMLogPublisher` instead of direct handler access to `builder/event`

Publishing is gated by:

- `config.AIGateway.EnableLLMLog`

### Event and MQ Wiring

- `builder/event/events.go`
- `builder/mq/types.go`

Added:

- `PublishLLMLogTrainingEvent(message []byte) error`
- `LLMLogTrainingSubject`
- `LLMLogTrainingGroup`

### Config

- `common/config/config.go`

Added AIGateway llmlog config:

- `AIGateway.EnableLLMLog`
- `OPENCSG_AIGATEWAY_LLMLOG_ENABLE`

Added `LLMLog` config:

- `Bucket`
- `Prefix`
- `WorkerNum`
- `BatchSize`
- `FlushIntervalSeconds`

`LLMLog.WorkerNum` controls the internal worker pool size in the `llmlog` consumer. MQ messages are parsed, dispatched into worker jobs, and ACKed only after a worker successfully appends the record.

### LLMLog Service

- `cmd/csghub-server/cmd/llmlog/llmlog.go`
- `cmd/csghub-server/cmd/llmlog/launch.go`
- `cmd/csghub-server/cmd/root.go`

Added a standalone `llmlog` command and registered it in the server root command.

Service startup now initializes:

- config
- OpenTelemetry
- MQ factory
- S3 client
- training writer
- training consumer

### LLMLog Consumer and Writer

- `llmlog/component/training_writer.go`
- `llmlog/component/training_writer_test.go`
- `llmlog/consumer/training.go`
- `llmlog/consumer/training_test.go`

Implemented:

- batched JSONL writing
- periodic flush
- S3 object upload
- MQ consumption for training events
- internal worker pool driven by `LLMLog.WorkerNum`
- worker startup in `Run()`
- worker panic recovery and restart
- ACK only after worker append succeeds

### Instrumentation

- `builder/instrumentation/types.go`

Added:

- `instrumentation.LLMLog`

## Current Behavior

When `OPENCSG_AIGATEWAY_LLMLOG_ENABLE=true`:

1. A chat-completion request enters AIGateway.
2. A training capture is initialized with `request_id`, model information, tools, request messages, and metadata.
3. The downstream response is proxied as before.
4. The response wrapper captures the final assistant output.
5. A normalized `LLMTrainingLogRecord` is built, with the model output merged into `messages`.
6. AIGateway publishes the record payload to MQ.
7. `llmlog` consumes the message, dispatches it into an internal worker pool, batches it, and writes JSONL part files to object storage.

## Example Record

```json
{
  "record_type": "training",
  "request_id": "trace-9f6b2f2c5f6a4c8b9c1d",
  "event_time": "2026-04-07T15:02:11.245+08:00",
  "sample_type": "chat_completion",
  "model_id": "Qwen/Qwen3-32B",
  "user_uuid": "user-123",
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "realtime_aqi",
        "description": "Get realtime AQI for a city.",
        "parameters": {
          "type": "object",
          "properties": {
            "city": {
              "type": "string"
            }
          },
          "required": [
            "city"
          ]
        }
      }
    }
  ],
  "messages": [
    {
      "role": "user",
      "content": "查一下北京和上海今天空气质量"
    },
    {
      "role": "tool_call",
      "content": "{\"name\":\"realtime_aqi\",\"arguments\":{\"city\":\"北京\"}}"
    },
    {
      "role": "tool_call",
      "content": "{\"name\":\"realtime_aqi\",\"arguments\":{\"city\":\"上海\"}}"
    },
    {
      "role": "tool_response",
      "content": "{\"city\":\"北京\",\"aqi\":10}"
    },
    {
      "role": "tool_response",
      "content": "{\"city\":\"上海\",\"aqi\":72}"
    },
    {
      "role": "assistant",
      "content": "北京空气质量优，上海轻度污染。"
    }
  ],
  "usage": {
    "prompt_tokens": 145,
    "completion_tokens": 52,
    "total_tokens": 197
  },
  "metadata": {
    "source": "aigateway",
    "api": "/v1/chat/completions",
    "stream": false,
    "provider": "opencsg",
    "svc_name": "qwen3-32b-infer",
    "finish_reason": "stop"
  }
}
```

This object is emitted as one JSON line in the final `.jsonl` file.

Example tool-use sample in the shipped schema:

```json
{
  "record_type": "training",
  "request_id": "04d66eca4abd481fa506a480319e5479",
  "event_time": "2026-04-07T15:16:37.048373Z",
  "sample_type": "chat_completion",
  "model_id": "deepseek-chat",
  "user_uuid": "1413e7f5-5bf3-4614-bf35-0e8be73747ee",
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "Get the current weather in a given location",
        "parameters": {
          "type": "object",
          "properties": {
            "location": {
              "type": "string",
              "description": "City and region, e.g. San Francisco, CA"
            },
            "unit": {
              "type": "string",
              "enum": [
                "celsius",
                "fahrenheit"
              ]
            }
          },
          "required": [
            "location"
          ]
        }
      }
    }
  ],
  "messages": [
    {
      "role": "system",
      "content": "You are a travel assistant. Prefer calling tools for live data (weather, flights). Be concise and cite tool outputs."
    },
    {
      "role": "user",
      "content": "I am flying SFO to NRT next Tuesday for work. First, what is the weather in San Francisco today? Then suggest whether I should pack a light jacket for the return leg based on typical SF fog."
    },
    {
      "role": "tool_call",
      "content": "{\"arguments\":\"{\\\"location\\\":\\\"San Francisco, CA\\\",\\\"unit\\\":\\\"celsius\\\"}\",\"name\":\"get_weather\"}"
    },
    {
      "role": "tool_response",
      "content": "{\"location\":\"San Francisco, CA\",\"temperature_c\":16,\"condition\":\"foggy\",\"summary\":\"Cool and foggy near the coast.\"}"
    },
    {
      "role": "user",
      "content": "Given that tool result, one sentence: should I pack a light jacket for when I fly back into SFO?"
    },
    {
      "role": "assistant",
      "content": "Yes, pack a light jacket - San Francisco is currently 16°C and foggy, and coastal fog is typical year-round."
    }
  ],
  "usage": {
    "prompt_tokens": 546,
    "completion_tokens": 27,
    "total_tokens": 573
  },
  "metadata": {
    "api": "/v1/chat/completions",
    "finish_reason": "stop",
    "provider": "deepseek",
    "source": "aigateway",
    "stream": true,
    "svc_name": ""
  }
}
```

## Verification

Executed successfully:

```bash
GOCACHE=/tmp/go-build go test ./llmlog/...
GOCACHE=/tmp/go-build go test ./aigateway/handler -run 'Test(TrainingCapture|NormalizeTrainingMessages|StringifyToolArguments)'
GOCACHE=/tmp/go-build go test -run '^$' ./aigateway/handler
GOCACHE=/tmp/go-build go test -run '^$' ./llmlog/... ./common/types
GOCACHE=/tmp/go-build go test -run '^$' ./cmd/csghub-server/cmd ./cmd/csghub-server/cmd/llmlog ./builder/event ./builder/mq ./common/config ./common/types ./builder/instrumentation
```

Constraint:

The full `go test ./aigateway/handler` suite was not run in this sandbox because some existing tests use local socket binding through `httptest.NewServer`, which is blocked in the current environment.

## Follow-Up Options

Likely V2 work:

- add query API to `llmlog`
- add indexing for request/model/user/time filtering
- optionally add non-training logs for observability and replay
- extend beyond chat completions to embeddings and image generation if needed
