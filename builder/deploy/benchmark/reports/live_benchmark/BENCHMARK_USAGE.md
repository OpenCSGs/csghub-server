# Deploy Benchmark Usage

## 1. Input File

- Input files are located in `builder/deploy/benchmark/reports/live_benchmark/inputs/`.
- Recommended for this round:
  - `sample.json`: SSE streaming scenario, validates `ttft_ms`.

## 2. Running with Python

```bash
NO_PROXY=localhost,127.0.0.1 no_proxy=localhost,127.0.0.1 \
python3 builder/deploy/benchmark/scripts/deploy_benchmark.py \
  < builder/deploy/benchmark/reports/live_benchmark/inputs/sample.json \
  > builder/deploy/benchmark/reports/live_benchmark/python_results_round2/local_8094_stream_python_result.json \
  2> builder/deploy/benchmark/reports/live_benchmark/python_results_round2/local_8094_stream_python_stderr.txt
```

## 3. Running with Go (live benchmark UT)

```bash
RUN_LIVE_BENCHMARK=1 \
LIVE_BENCHMARK_OUTPUT_SUBDIR=go_results_round2 \
go test ./builder/deploy/benchmark -tags live_benchmark -run TestRunner_RunLiveBenchmarkCases -v
```

- `RUN_LIVE_BENCHMARK=1`: Explicitly enables real URL benchmark testing.
- `LIVE_BENCHMARK_OUTPUT_SUBDIR`: Specifies the output subdirectory name for preserving results across multiple test rounds.

## 4. Output Directory

- Python results: `builder/deploy/benchmark/reports/live_benchmark/python_results_round2/`
- Go results: `builder/deploy/benchmark/reports/live_benchmark/go_results_round2/`
- Comparison reports:
  - `benchmark_comparison_round2.md`
  - `benchmark_comparison_round2.json`
