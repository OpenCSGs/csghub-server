"""Unit tests for Whisper translation capability detection (no FunASR runtime needed)."""

from __future__ import annotations

import importlib.util
import sys
import unittest
from pathlib import Path
from unittest.mock import MagicMock


def _load_server_module():
    """Import server.py with heavy optional deps stubbed out."""
    for name in ("funasr", "uvicorn", "fastapi", "fastapi.responses"):
        sys.modules.setdefault(name, MagicMock())
    # FastAPI decorators must still return callables.
    fastapi_mock = sys.modules["fastapi"]
    fastapi_mock.FastAPI.return_value = MagicMock()
    fastapi_mock.File = MagicMock()
    fastapi_mock.Form = MagicMock()
    fastapi_mock.HTTPException = type("HTTPException", (Exception,), {})
    fastapi_mock.UploadFile = MagicMock()

    path = Path(__file__).resolve().parent / "server.py"
    spec = importlib.util.spec_from_file_location("funasr_server", path)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


server = _load_server_module()


class SupportsTranslationTests(unittest.TestCase):
    def test_multilingual_whisper_supports_translation(self):
        self.assertTrue(
            server.supports_translation("WhisperForConditionalGeneration", "openai/whisper-large-v3")
        )
        self.assertTrue(
            server.supports_translation("whisper", "openai/whisper-small")
        )

    def test_turbo_whisper_rejects_translation(self):
        self.assertFalse(
            server.supports_translation(
                "WhisperForConditionalGeneration", "openai/whisper-large-v3-turbo"
            )
        )
        self.assertFalse(
            server.supports_translation("whisper", "Systran/faster-whisper-large-v3-turbo")
        )

    def test_english_only_whisper_rejects_translation(self):
        self.assertFalse(
            server.supports_translation(
                "WhisperForConditionalGeneration", "openai/whisper-small.en"
            )
        )
        self.assertFalse(
            server.supports_translation("whisper", "openai/whisper-medium.en")
        )
        self.assertFalse(
            server.supports_translation("whisper", "whisper-tiny.en")
        )

    def test_non_whisper_rejects_translation(self):
        self.assertFalse(server.supports_translation("SenseVoiceSmall", "iic/SenseVoiceSmall"))
        self.assertFalse(server.supports_translation("", "openai/whisper-large-v3"))

    def test_is_english_only_whisper(self):
        self.assertTrue(server.is_english_only_whisper("openai/whisper-small.en"))
        self.assertFalse(server.is_english_only_whisper("openai/whisper-small"))
        self.assertFalse(server.is_english_only_whisper("openai/whisper-large-v3"))


if __name__ == "__main__":
    unittest.main()
