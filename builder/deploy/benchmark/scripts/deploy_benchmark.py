#!/usr/bin/env python3
"""
Deploy Benchmark Script - Fallback for TPM/RPM testing

This script provides a manual fallback for benchmark testing when
the automated Go-based system is unavailable.

Usage:
    python deploy_benchmark.py < input.json > output.json

Input JSON format:
{
    "endpoint": "https://...",
    "request_template": {
        "api_path": "/v1/chat/completions",
        "method": "POST",
        "headers": {"Content-Type": "application/json"},
        "request_body": {"model": "...", "messages": [...]}
    },
    "config": {
        "warmup_requests": 2,
        "duration_seconds": 60,
        "concurrency": 4,
        "timeout_seconds": 30,
        "enable_stream": false
    }
}

Output JSON format:
{
    "summary": {
        "total_requests": 120,
        "success_requests": 120,
        "failed_requests": 0,
        "success_rate": 1.0,
        "avg_latency_ms": 820.5,
        "p95_latency_ms": 1350.0,
        "p99_latency_ms": 1800.0,
        "ttft_ms": 0.0,
        "ttft_available": false,
        "prompt_tokens": 18000,
        "completion_tokens": 42000,
        "total_tokens": 60000,
        "tpm": 60000.0,
        "rps": 2.0
    },
    "raw_result": {...}
}
"""

import json
import math
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from concurrent.futures import ThreadPoolExecutor, as_completed


def build_url(endpoint: str, api_path: str) -> str:
    endpoint = (endpoint or "").rstrip("/")
    api_path = api_path or ""
    if not api_path.startswith("/"):
        api_path = "/" + api_path
    return endpoint + api_path


def extract_usage(payload: dict) -> dict:
    usage = payload.get("usage") or {}
    return {
        "prompt_tokens": int(usage.get("prompt_tokens", 0) or 0),
        "completion_tokens": int(usage.get("completion_tokens", 0) or 0),
        "total_tokens": int(usage.get("total_tokens", 0) or 0),
    }


def percentile(values, ratio: float) -> float:
    if not values:
        return 0.0
    ordered = sorted(values)
    index = int(math.ceil(len(ordered) * ratio)) - 1
    index = max(0, min(index, len(ordered) - 1))
    return float(ordered[index])


def request_once(url: str, method: str, headers: dict, body: dict, timeout_seconds: int, enable_stream: bool) -> dict:
    body_bytes = json.dumps(body).encode("utf-8")
    req = urllib.request.Request(url=url, data=body_bytes, headers=headers, method=method)
    start = time.perf_counter()
    try:
        with urllib.request.urlopen(req, timeout=timeout_seconds) as resp:
            status_code = int(getattr(resp, "status", 200))
            if status_code < 200 or status_code >= 300:
                response_bytes = resp.read()
                latency_ms = (time.perf_counter() - start) * 1000.0
                return {
                    "ok": False,
                    "status_code": status_code,
                    "latency_ms": latency_ms,
                    "ttft_ms": 0.0,
                    "ttft_available": False,
                    "error": response_bytes.decode("utf-8", errors="ignore"),
                    "usage": {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0},
                }

            if enable_stream:
                return parse_streaming_response(resp, start, status_code)

            response_bytes = resp.read()
            latency_ms = (time.perf_counter() - start) * 1000.0
            payload = json.loads(response_bytes.decode("utf-8") or "{}")
            usage = extract_usage(payload)
            # For non-streaming requests, TTFT is not applicable
            return {
                "ok": True,
                "status_code": status_code,
                "latency_ms": latency_ms,
                "ttft_ms": 0.0,
                "ttft_available": False,
                "usage": usage,
            }
    except urllib.error.HTTPError as exc:
        latency_ms = (time.perf_counter() - start) * 1000.0
        return {
            "ok": False,
            "status_code": exc.code,
            "latency_ms": latency_ms,
            "ttft_ms": 0.0,
            "ttft_available": False,
            "error": str(exc),
            "usage": {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0},
        }
    except Exception as exc:
        latency_ms = (time.perf_counter() - start) * 1000.0
        return {
            "ok": False,
            "status_code": 0,
            "latency_ms": latency_ms,
            "ttft_ms": 0.0,
            "ttft_available": False,
            "error": str(exc),
            "usage": {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0},
        }


def parse_sse_data(line: str) -> tuple[str, bool]:
    prefix = "data:"
    if not line.startswith(prefix):
        return "", False
    return line[len(prefix):].strip(), True


def has_usage(usage: dict) -> bool:
    return any((usage.get("prompt_tokens", 0), usage.get("completion_tokens", 0), usage.get("total_tokens", 0)))


def parse_streaming_response(resp, start_time: float, status_code: int) -> dict:
    ttft_ms = 0.0
    ttft_available = False
    usage = {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0}
    fallback_chunks = []

    while True:
        line = resp.readline()
        if not line:
            break
        decoded = line.decode("utf-8", errors="ignore").strip()
        if not decoded:
            continue
        fallback_chunks.append(decoded)

        event_data, is_sse = parse_sse_data(decoded)
        if is_sse:
            if not event_data or event_data == "[DONE]":
                continue
            if not ttft_available:
                ttft_available = True
                ttft_ms = (time.perf_counter() - start_time) * 1000.0
            try:
                payload = json.loads(event_data)
                candidate = extract_usage(payload)
                if has_usage(candidate):
                    usage = candidate
            except json.JSONDecodeError:
                continue
            continue

        try:
            payload = json.loads(decoded)
            candidate = extract_usage(payload)
            if has_usage(candidate):
                usage = candidate
        except json.JSONDecodeError:
            continue

    if not ttft_available and fallback_chunks:
        try:
            payload = json.loads("".join(fallback_chunks))
            candidate = extract_usage(payload)
            if has_usage(candidate):
                usage = candidate
        except json.JSONDecodeError:
            pass

    latency_ms = (time.perf_counter() - start_time) * 1000.0
    return {
        "ok": True,
        "status_code": status_code,
        "latency_ms": latency_ms,
        "ttft_ms": ttft_ms,
        "ttft_available": ttft_available,
        "usage": usage,
    }


def run_benchmark(input_data: dict) -> dict:
    endpoint = input_data["endpoint"]
    request_template = input_data["request_template"]
    config = input_data["config"]
    url = build_url(endpoint, request_template["api_path"])
    method = request_template.get("method", "POST")
    headers = request_template.get("headers", {"Content-Type": "application/json"})
    body = request_template.get("request_body", {})
    timeout_seconds = int(config.get("timeout_seconds", 30))
    warmup_requests = int(config.get("warmup_requests", 2))
    duration_seconds = int(config.get("duration_seconds", 60))
    concurrency = int(config.get("concurrency", 2))
    enable_stream = bool(config.get("enable_stream", False))

    # Warmup phase - sequential requests
    for _ in range(max(0, warmup_requests)):
        result = request_once(url, method, headers, body, timeout_seconds, enable_stream)
        if not result["ok"]:
            first_error = result.get("error", "warmup failed")
            raise RuntimeError(f"warmup failed: {first_error}")

    # Benchmark phase - fixed number of worker threads, each sending requests in loop
    started_at = time.time()
    deadline = started_at + max(duration_seconds, 1)
    results = []

    def worker_loop():
        worker_results = []
        while time.time() < deadline:
            result = request_once(url, method, headers, body, timeout_seconds, enable_stream)
            worker_results.append(result)
        return worker_results

    with ThreadPoolExecutor(max_workers=concurrency) as executor:
        futures = [executor.submit(worker_loop) for _ in range(concurrency)]
        for future in as_completed(futures):
            try:
                worker_results = future.result()
                results.extend(worker_results)
            except Exception as e:
                results.append({
                    "ok": False,
                    "status_code": 0,
                    "latency_ms": 0,
                    "ttft_ms": 0,
                    "ttft_available": False,
                    "error": str(e),
                    "usage": {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0},
                })

    finished_at = time.time()
    duration = max(finished_at - started_at, 1e-6)
    latencies = [item["latency_ms"] for item in results]
    ttfts = [item["ttft_ms"] for item in results if item.get("ttft_available", False)]
    ttft_available_count = len(ttfts)
    success_requests = sum(1 for item in results if item["ok"])
    failed_requests = len(results) - success_requests
    prompt_tokens = sum(item["usage"]["prompt_tokens"] for item in results)
    completion_tokens = sum(item["usage"]["completion_tokens"] for item in results)
    total_tokens = sum(item["usage"]["total_tokens"] for item in results)

    # TTFT is only meaningful for streaming requests
    ttft_ms = 0.0
    ttft_available = enable_stream and ttft_available_count > 0
    if ttft_available and ttfts:
        ttft_ms = sum(ttfts) / len(ttfts)

    summary = {
        "total_requests": len(results),
        "success_requests": success_requests,
        "failed_requests": failed_requests,
        "success_rate": float(success_requests) / float(len(results)) if results else 0.0,
        "avg_latency_ms": float(sum(latencies) / len(latencies)) if latencies else 0.0,
        "p95_latency_ms": percentile(latencies, 0.95),
        "p99_latency_ms": percentile(latencies, 0.99),
        "ttft_ms": ttft_ms,
        "ttft_available": ttft_available,
        "prompt_tokens": prompt_tokens,
        "completion_tokens": completion_tokens,
        "total_tokens": total_tokens,
        "tpm": float(total_tokens) * 60.0 / duration if duration > 0 else 0.0,
        "rps": float(len(results)) / duration if duration > 0 else 0.0,
    }

    return {
        "summary": summary,
        "raw_result": {
            "url": url,
            "duration_seconds": duration,
            "warmup_requests": warmup_requests,
            "concurrency": concurrency,
            "enable_stream": enable_stream,
            "sample_count": len(results),
            "ttft_available": ttft_available,
            "ttft_sample_count": ttft_available_count,
            "errors": [item.get("error", "") for item in results if not item["ok"]][:20],
        },
    }


def main() -> int:
    try:
        input_data = json.load(sys.stdin)
        result = run_benchmark(input_data)
        sys.stdout.write(json.dumps(result))
        return 0
    except Exception as exc:
        sys.stderr.write(str(exc))
        return 1


if __name__ == "__main__":
    sys.exit(main())
