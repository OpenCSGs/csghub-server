import argparse
import json
import logging
import os
import re
import subprocess
import tempfile
import time
from pathlib import Path
from typing import Optional

import uvicorn
from fastapi import FastAPI, File, Form, HTTPException, UploadFile
from fastapi.responses import JSONResponse, PlainTextResponse, StreamingResponse
from funasr import AutoModel


logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
logger = logging.getLogger(__name__)

app = FastAPI(title="FunASR OpenAI-Compatible API", version="1.0.0")

ASR_MODEL = None
MODEL_ID = "local"
MODEL_PATH = ""
DEVICE = "cpu"
VAD_MODEL = ""
VAD_MAX_SINGLE_SEGMENT_TIME = 30000
BATCH_SIZE = 1
BATCH_SIZE_S = 60.0
BATCH_SIZE_THRESHOLD_S = 30.0
STREAM_CHUNK_SECONDS = 30.0


def env_bool(name: str, default: bool = False) -> bool:
    value = os.getenv(name)
    if value is None:
        return default
    return value.lower() in {"1", "true", "yes", "on"}


def env_int(name: str, default: int) -> int:
    value = os.getenv(name)
    if value is None or value == "":
        return default
    return int(value)


def env_float(name: str, default: float) -> float:
    value = os.getenv(name)
    if value is None or value == "":
        return default
    return float(value)


def clean_text(text: str) -> str:
    return re.sub(r"<\|[^|]*\|>", "", text).strip()


def normalize_optional_model(value: Optional[str]) -> str:
    if not value:
        return ""
    value = value.strip()
    if value.lower() in {"0", "false", "none", "off", "no"}:
        return ""
    return value


def load_local_model(model_path: str, device: str, vad_model: str, vad_max_single_segment_time: int):
    path = Path(model_path)
    if not path.exists():
        raise FileNotFoundError(f"local model path does not exist: {model_path}")

    logger.info("Loading FunASR local model from %s on %s", model_path, device)
    start = time.time()
    model_kwargs = {
        "model": model_path,
        "device": device,
        "disable_update": True,
        "trust_remote_code": env_bool("FUNASR_TRUST_REMOTE_CODE"),
    }
    if vad_model:
        model_kwargs["vad_model"] = vad_model
        model_kwargs["vad_kwargs"] = {"max_single_segment_time": vad_max_single_segment_time}
        logger.info(
            "FunASR VAD enabled: vad_model=%s max_single_segment_time=%sms",
            vad_model,
            vad_max_single_segment_time,
        )

    model = AutoModel(**model_kwargs)
    logger.info("Loaded FunASR local model in %.1fs", time.time() - start)
    return model


def build_generate_kwargs(input_paths: list[str], language: Optional[str], hotwords: Optional[str]) -> dict:
    generate_kwargs = {
        "input": input_paths,
        "cache": {},
        "batch_size": BATCH_SIZE,
    }
    if BATCH_SIZE_S > 0:
        generate_kwargs["batch_size_s"] = BATCH_SIZE_S
    if BATCH_SIZE_THRESHOLD_S > 0:
        generate_kwargs["batch_size_threshold_s"] = BATCH_SIZE_THRESHOLD_S
    if language:
        generate_kwargs["language"] = language
    if hotwords:
        generate_kwargs["hotwords"] = [word.strip() for word in hotwords.split(",") if word.strip()]
    return generate_kwargs


def transcribe_files(input_paths: list[str], language: Optional[str], hotwords: Optional[str]):
    generate_kwargs = build_generate_kwargs(input_paths, language, hotwords)
    return ASR_MODEL.generate(**generate_kwargs)


def split_audio(input_path: str, output_dir: str) -> list[str]:
    chunk_seconds = max(STREAM_CHUNK_SECONDS, 1.0)
    output_pattern = os.path.join(output_dir, "chunk_%05d.wav")
    command = [
        "ffmpeg",
        "-nostdin",
        "-hide_banner",
        "-loglevel",
        "error",
        "-i",
        input_path,
        "-vn",
        "-map",
        "0:a:0",
        "-f",
        "segment",
        "-segment_time",
        str(chunk_seconds),
        "-reset_timestamps",
        "1",
        "-acodec",
        "pcm_s16le",
        output_pattern,
    ]
    subprocess.run(command, check=True)
    chunks = sorted(str(path) for path in Path(output_dir).glob("chunk_*.wav"))
    return chunks or [input_path]


def format_stream_chunk(text: str, response_format: Optional[str]) -> str:
    if response_format in {"json", "verbose_json"}:
        return json.dumps({"text": text}, ensure_ascii=False) + "\n"
    return text + "\n"


def stream_transcription(
    input_path: str,
    language: Optional[str],
    hotwords: Optional[str],
    response_format: Optional[str],
):
    try:
        with tempfile.TemporaryDirectory(prefix="funasr-stream-") as chunk_dir:
            chunks = split_audio(input_path, chunk_dir)
            total_chunks = len(chunks)
            for index, chunk_path in enumerate(chunks, start=1):
                start = time.time()
                result = transcribe_files([chunk_path], language, hotwords)
                elapsed = time.time() - start
                first_result = result[0] if result else {}
                text = clean_text(first_result.get("text", ""))
                logger.info("Streamed FunASR chunk %s/%s in %.1fs", index, total_chunks, elapsed)
                if text:
                    yield format_stream_chunk(text, response_format)
    except Exception as exc:
        logger.exception("Streaming transcription error")
        yield format_stream_chunk(f"[ERROR] {exc}", response_format)
    finally:
        if os.path.exists(input_path):
            os.unlink(input_path)


@app.post("/v1/audio/transcriptions")
async def transcribe(
    file: UploadFile = File(...),
    model: str = Form(default="local"),
    language: Optional[str] = Form(default=None),
    hotwords: Optional[str] = Form(default=None),
    response_format: Optional[str] = Form(default="json"),
    stream: bool = Form(default=True),
):
    if ASR_MODEL is None:
        raise HTTPException(status_code=503, detail="model is not loaded")

    suffix = os.path.splitext(file.filename)[1] if file.filename else ".wav"
    tmp_path = ""
    try:
        with tempfile.NamedTemporaryFile(delete=False, suffix=suffix) as tmp:
            content = await file.read()
            tmp.write(content)
            tmp_path = tmp.name

        if stream:
            tmp_path_for_stream = tmp_path
            tmp_path = ""
            media_type = "application/x-ndjson" if response_format in {"json", "verbose_json"} else "text/plain"
            return StreamingResponse(
                stream_transcription(tmp_path_for_stream, language, hotwords, response_format),
                media_type=media_type,
            )

        start = time.time()
        result = transcribe_files([tmp_path], language, hotwords)
        elapsed = time.time() - start

        first_result = result[0] if result else {}
        text = clean_text(first_result.get("text", ""))
        if response_format == "text":
            return PlainTextResponse(text)

        if response_format == "verbose_json":
            segments = []
            for segment in first_result.get("sentence_info", []):
                segments.append(
                    {
                        "start": segment.get("start", 0) / 1000.0,
                        "end": segment.get("end", 0) / 1000.0,
                        "text": clean_text(segment.get("text", "")),
                        "speaker": segment.get("spk"),
                    }
                )
            return JSONResponse(
                {
                    "text": text,
                    "segments": segments,
                    "language": language or "auto",
                    "duration": round(elapsed, 3),
                    "model": model or MODEL_ID,
                }
            )

        return JSONResponse({"text": text})
    except Exception as exc:
        logger.exception("Transcription error")
        raise HTTPException(status_code=500, detail=str(exc)) from exc
    finally:
        if tmp_path and os.path.exists(tmp_path):
            os.unlink(tmp_path)


@app.get("/v1/models")
async def list_models():
    aliases = ["local", MODEL_ID]
    repo_name = MODEL_ID.rsplit("/", 1)[-1]
    if repo_name not in aliases:
        aliases.append(repo_name)

    return JSONResponse(
        {
            "object": "list",
            "data": [
                {
                    "id": alias,
                    "object": "model",
                    "created": 1700000000,
                    "owned_by": "funasr",
                    "ready": ASR_MODEL is not None,
                }
                for alias in aliases
            ],
        }
    )


@app.get("/health")
async def health():
    return {
        "status": "ok" if ASR_MODEL is not None else "loading",
        "device": DEVICE,
        "model_id": MODEL_ID,
        "model_path": MODEL_PATH,
        "vad_model": VAD_MODEL or None,
        "vad_max_single_segment_time": VAD_MAX_SINGLE_SEGMENT_TIME,
        "batch_size": BATCH_SIZE,
        "batch_size_s": BATCH_SIZE_S,
        "batch_size_threshold_s": BATCH_SIZE_THRESHOLD_S,
        "stream_chunk_seconds": STREAM_CHUNK_SECONDS,
        "models_loaded": [MODEL_ID] if ASR_MODEL is not None else [],
    }


def main():
    parser = argparse.ArgumentParser(description="FunASR local model API server")
    parser.add_argument("--host", default="0.0.0.0", help="Bind host")
    parser.add_argument("--port", type=int, default=8000, help="Bind port")
    parser.add_argument("--device", default="cpu", help="Device: cuda, cpu, mps")
    parser.add_argument("--model-path", required=True, help="Local model directory")
    parser.add_argument("--model-id", default="local", help="Model id exposed by the API")
    parser.add_argument(
        "--vad-model",
        default=os.getenv("FUNASR_VAD_MODEL", "fsmn-vad"),
        help="FunASR VAD model for long audio/video chunking. Use 'none' to disable.",
    )
    parser.add_argument(
        "--vad-max-single-segment-time",
        type=int,
        default=env_int("FUNASR_VAD_MAX_SINGLE_SEGMENT_TIME", 30000),
        help="Maximum VAD segment length in milliseconds",
    )
    parser.add_argument(
        "--batch-size",
        type=int,
        default=env_int("FUNASR_BATCH_SIZE", 1),
        help="FunASR batch_size",
    )
    parser.add_argument(
        "--batch-size-s",
        type=float,
        default=env_float("FUNASR_BATCH_SIZE_S", 60.0),
        help="Dynamic batch size by total audio duration in seconds. Use 0 to disable.",
    )
    parser.add_argument(
        "--batch-size-threshold-s",
        type=float,
        default=env_float("FUNASR_BATCH_SIZE_THRESHOLD_S", 30.0),
        help="Force batch size 1 for VAD segments longer than this many seconds. Use 0 to disable.",
    )
    args, unknown_args = parser.parse_known_args()
    if unknown_args:
        logger.warning("Ignoring unsupported server args: %s", unknown_args)

    global ASR_MODEL, DEVICE, MODEL_ID, MODEL_PATH
    global VAD_MODEL, VAD_MAX_SINGLE_SEGMENT_TIME, BATCH_SIZE
    global BATCH_SIZE_S, BATCH_SIZE_THRESHOLD_S, STREAM_CHUNK_SECONDS
    DEVICE = args.device
    MODEL_ID = args.model_id
    MODEL_PATH = args.model_path
    VAD_MODEL = normalize_optional_model(args.vad_model)
    VAD_MAX_SINGLE_SEGMENT_TIME = args.vad_max_single_segment_time
    BATCH_SIZE = args.batch_size
    BATCH_SIZE_S = args.batch_size_s
    BATCH_SIZE_THRESHOLD_S = args.batch_size_threshold_s
    STREAM_CHUNK_SECONDS = env_float("FUNASR_STREAM_CHUNK_SECONDS", 30.0)
    ASR_MODEL = load_local_model(
        args.model_path,
        args.device,
        VAD_MODEL,
        VAD_MAX_SINGLE_SEGMENT_TIME,
    )

    logger.info("FunASR API server starting on http://%s:%s", args.host, args.port)
    uvicorn.run(app, host=args.host, port=args.port)


if __name__ == "__main__":
    main()
