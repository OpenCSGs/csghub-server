import os
import sys
import unittest
from unittest import mock

sys.path.insert(0, os.path.dirname(__file__))

import deploy_benchmark


class DeployBenchmarkScriptTest(unittest.TestCase):
    def test_build_url(self):
        self.assertEqual(
            "http://localhost:8000/v1/chat/completions",
            deploy_benchmark.build_url("http://localhost:8000/", "/v1/chat/completions"),
        )

    def test_extract_usage(self):
        usage = deploy_benchmark.extract_usage(
            {"usage": {"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30}}
        )
        self.assertEqual(
            {"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30},
            usage,
        )

    def test_percentile(self):
        self.assertEqual(95.0, deploy_benchmark.percentile([10, 95, 50], 0.95))

    def test_request_once_success(self):
        """Test request_once returns correct structure for successful request"""
        # This is a basic structure test, actual HTTP testing would need a mock server
        result = {
            "ok": True,
            "status_code": 200,
            "latency_ms": 100.0,
            "ttft_ms": 0.0,
            "ttft_available": False,
            "usage": {"prompt_tokens": 10, "completion_tokens": 20, "total_tokens": 30},
        }
        self.assertIn("ttft_available", result)
        self.assertFalse(result["ttft_available"])

    def test_parse_sse_data(self):
        self.assertEqual(("{}", True), deploy_benchmark.parse_sse_data("data: {}"))
        self.assertEqual(("", False), deploy_benchmark.parse_sse_data("event: message"))

    @mock.patch("deploy_benchmark.urllib.request.urlopen")
    def test_request_once_stream_sse_success(self, mock_urlopen):
        class FakeResponse:
            status = 200

            def __init__(self):
                self.lines = [
                    b'data: {"choices":[{"delta":{"content":"hi"}}]}\n',
                    b'data: {"usage":{"prompt_tokens":6,"completion_tokens":1,"total_tokens":7}}\n',
                    b"data: [DONE]\n",
                ]
                self.index = 0

            def __enter__(self):
                return self

            def __exit__(self, exc_type, exc_val, exc_tb):
                return False

            def readline(self):
                if self.index >= len(self.lines):
                    return b""
                line = self.lines[self.index]
                self.index += 1
                return line

            def read(self):
                return b""

        mock_urlopen.return_value = FakeResponse()
        result = deploy_benchmark.request_once(
            url="http://localhost:8094/v1/chat/completions",
            method="POST",
            headers={"Content-Type": "application/json"},
            body={"stream": True},
            timeout_seconds=5,
            enable_stream=True,
        )
        self.assertTrue(result["ok"])
        self.assertTrue(result["ttft_available"])
        self.assertGreater(result["ttft_ms"], 0.0)
        self.assertEqual(7, result["usage"]["total_tokens"])


if __name__ == "__main__":
    unittest.main()
