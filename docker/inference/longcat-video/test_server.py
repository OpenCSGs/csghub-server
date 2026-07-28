import os
import sys
import tempfile
import types
import unittest
from pathlib import Path
from unittest import mock

os.environ.setdefault("REPO_ID", "meituan-longcat/LongCat-Video-Avatar-1.5")

try:
    import fastapi  # noqa: F401
except ModuleNotFoundError:
    class FakeHTTPException(Exception):
        def __init__(self, status_code: int, detail: str) -> None:
            super().__init__(detail)
            self.status_code = status_code
            self.detail = detail

    class FakeFastAPI:
        def __init__(self, *args, **kwargs) -> None:
            pass

        def _decorator(self, *args, **kwargs):
            return lambda func: func

        post = _decorator
        get = _decorator
        on_event = _decorator

    fake_fastapi = types.ModuleType("fastapi")
    fake_fastapi.FastAPI = FakeFastAPI
    fake_fastapi.HTTPException = FakeHTTPException
    fake_fastapi.Request = object
    fake_responses = types.ModuleType("fastapi.responses")
    fake_responses.FileResponse = object
    sys.modules["fastapi"] = fake_fastapi
    sys.modules["fastapi.responses"] = fake_responses

import server
from fastapi import HTTPException


class TaskPersistenceTest(unittest.TestCase):
    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp_dir.cleanup)
        self.old_state_dir = server.TASK_STATE_DIR
        self.old_output_dir = server.OUTPUT_DIR
        server.TASK_STATE_DIR = Path(self.temp_dir.name) / "states"
        server.OUTPUT_DIR = Path(self.temp_dir.name) / "videos"

    def tearDown(self) -> None:
        server.TASK_STATE_DIR = self.old_state_dir
        server.OUTPUT_DIR = self.old_output_dir

    def make_task(self, status: str = "submitted") -> server.Task:
        task_id = "0123456789abcdef0123456789abcdef"
        work_dir = Path(self.temp_dir.name) / "work"
        return server.Task(
            task_id=task_id,
            work_dir=work_dir,
            input_json=work_dir / "input.json",
            output_dir=work_dir / "generated",
            audio_count=1,
            resolution="480p",
            num_segments=1,
            ref_img_index=10,
            mask_frame_range=3,
            status=status,
        )

    def test_persists_terminal_state_and_download_url(self) -> None:
        task = self.make_task("succeed")
        task.object_key = f"aigateway/generated/videos/{task.task_id}.mp4"
        task.download_url = f"https://video.example.com/{task.object_key}"

        server._persist_task(task)
        state = server._read_task_state(task.task_id)

        self.assertEqual("succeed", state["status"])
        self.assertEqual(task.object_key, state["object_key"])
        self.assertEqual(task.download_url, state["download_url"])
        response = server.task_status(task.task_id)
        self.assertEqual(task.download_url, response["download_url"])
        self.assertNotIn("object_key", response)

    def test_recovery_marks_unfinished_task_failed(self) -> None:
        task = self.make_task("running")
        server._persist_task(task)

        server._recover_task_states()
        state = server._read_task_state(task.task_id)

        self.assertEqual("failed", state["status"])
        self.assertIn("interrupted", state["message"])

    def test_invalid_or_corrupt_state_is_not_found(self) -> None:
        with self.assertRaises(HTTPException) as invalid:
            server._read_task_state("../not-a-task")
        self.assertEqual(404, invalid.exception.status_code)

        task = self.make_task()
        server.TASK_STATE_DIR.mkdir(parents=True)
        (server.TASK_STATE_DIR / f"{task.task_id}.json").write_text("{", encoding="utf-8")
        with self.assertRaises(HTTPException) as corrupt:
            server._read_task_state(task.task_id)
        self.assertEqual(404, corrupt.exception.status_code)

    def test_upload_failure_preserves_successful_local_video(self) -> None:
        task = self.make_task()
        task.output_dir.mkdir(parents=True)
        generated = task.output_dir / "generated.mp4"
        generated.write_bytes(b"video")

        with (
            mock.patch.object(server, "build_command", return_value=["inference"]),
            mock.patch.object(server.subprocess, "run"),
            mock.patch.object(server, "_s3_upload_enabled", return_value=True),
            mock.patch.object(server, "_upload_video", side_effect=RuntimeError("OSS unavailable")),
        ):
            server.run_task(task)

        state = server._read_task_state(task.task_id)
        self.assertEqual("succeed", state["status"])
        self.assertNotIn("error", state)
        self.assertNotIn("download_url", state)
        self.assertEqual(
            b"video",
            (server.OUTPUT_DIR / f"{task.task_id}.mp4").read_bytes(),
        )
        self.assertEqual(
            f"/v1/files/download/outputs/videos/{task.task_id}.mp4",
            server.task_status(task.task_id)["output"],
        )


class ObjectStorageTest(unittest.TestCase):
    def setUp(self) -> None:
        self.original = {
            name: getattr(server, name)
            for name in (
                "S3_ACCESS_ID",
                "S3_ACCESS_SECRET",
                "S3_BUCKET",
                "S3_ENDPOINT",
                "S3_SSL_ENABLED",
            )
        }

    def tearDown(self) -> None:
        for name, value in self.original.items():
            setattr(server, name, value)

    def test_empty_bucket_disables_upload(self) -> None:
        server.S3_ACCESS_ID = "id"
        server.S3_ACCESS_SECRET = "secret"
        server.S3_ENDPOINT = "minio.example.com"
        server.S3_BUCKET = ""
        self.assertFalse(server._s3_upload_enabled())

    def test_minio_upload_returns_stable_url(self) -> None:
        server.S3_ACCESS_ID = "id"
        server.S3_ACCESS_SECRET = "secret"
        server.S3_ENDPOINT = "minio.example.com"
        server.S3_BUCKET = "video-bucket"
        server.S3_SSL_ENABLED = True
        calls: list[tuple[str, str, str]] = []

        class FakeClient:
            def fput_object(self, bucket: str, key: str, path: str, content_type: str) -> None:
                self.content_type = content_type
                calls.append((bucket, key, path))

        fake_module = types.SimpleNamespace(Minio=lambda *args, **kwargs: FakeClient())
        with tempfile.NamedTemporaryFile(suffix=".mp4") as video:
            with mock.patch.dict(sys.modules, {"minio": fake_module}):
                key, download_url = server._upload_video(Path(video.name), "task123")

        self.assertEqual("aigateway/generated/videos/task123.mp4", key)
        self.assertEqual(("video-bucket", key, video.name), calls[0])
        self.assertEqual(f"https://minio.example.com/video-bucket/{key}", download_url)


if __name__ == "__main__":
    unittest.main()
