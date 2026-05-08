# Benchmark Result Fields

## Top-level Structure

- `summary`: Aggregated metrics.
- `raw_result`: Runtime metadata and error samples.

## summary Fields

- `total_requests`: Total requests during the benchmark phase (excluding warmup).
- `success_requests`: Number of successful requests (HTTP success and parsing success).
- `failed_requests`: Number of failed requests.
- `success_rate`: `success_requests / total_requests`.
- `avg_latency_ms`: Average total request latency in milliseconds.
- `p95_latency_ms`: P95 total latency in milliseconds.
- `p99_latency_ms`: P99 total latency in milliseconds.
- `ttft_ms`: Average time-to-first-token in milliseconds, meaningful only for streaming requests.
- `ttft_available`: Whether TTFT was successfully sampled.
- `prompt_tokens`: Total input token count.
- `completion_tokens`: Total output token count.
- `total_tokens`: Total token count.
- `tpm`: Token throughput per minute (`total_tokens * 60 / duration_seconds`).
- `rps`: Requests per second (`total_requests / duration_seconds`).

## raw_result Fields

- `url`: Final request URL.
- `duration_seconds`: Actual duration of the benchmark phase in seconds.
- `warmup_requests`: Number of warmup requests.
- `concurrency`: Number of concurrent workers.
- `enable_stream`: Whether streaming mode was enabled.
- `sample_count`: Number of sampled requests (usually equals `total_requests`).
- `ttft_available`: Whether TTFT samples are available.
- `ttft_sample_count`: Number of TTFT samples.
- `errors`: Error summary for failed samples (up to the first 20 entries).

## Streaming (SSE) Notes

- Streaming mode parses `data: ...` event lines.
- The arrival time of the first valid `data` event is recorded as the per-request TTFT.
- If the event includes a `usage` field, it is used for token accounting; otherwise, the request tokens are counted as 0.
- A `data: [DONE]` line is treated as the stream end marker.
