import argparse
import logging
import os
import re
import tempfile
import time
from pathlib import Path
from typing import Optional

import uvicorn
from fastapi import FastAPI, File, Form, HTTPException, UploadFile
from fastapi.responses import JSONResponse, PlainTextResponse
from funasr import AutoModel


logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
logger = logging.getLogger(__name__)

app = FastAPI(title="FunASR OpenAI-Compatible API", version="1.0.0")

ASR_MODEL = None
MODEL_ID = "local"
MODEL_PATH = ""
DEVICE = "cpu"


def env_bool(name: str, default: bool = False) -> bool:
    value = os.getenv(name)
    if value is None:
        return default
    return value.lower() in {"1", "true", "yes", "on"}


def clean_text(text: str) -> str:
    return re.sub(r"<\|[^|]*\|>", "", text).strip()


def load_local_model(model_path: str, device: str):
    path = Path(model_path)
    if not path.exists():
        raise FileNotFoundError(f"local model path does not exist: {model_path}")

    logger.info("Loading FunASR local model from %s on %s", model_path, device)
    start = time.time()
    model = AutoModel(
        model=model_path,
        device=device,
        disable_update=True,
        trust_remote_code=env_bool("FUNASR_TRUST_REMOTE_CODE"),
    )
    logger.info("Loaded FunASR local model in %.1fs", time.time() - start)
    return model


@app.post("/v1/audio/transcriptions")
async def transcribe(
    file: UploadFile = File(...),
    model: str = Form(default="local"),
    language: Optional[str] = Form(default=None),
    response_format: Optional[str] = Form(default="json"),
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

        generate_kwargs = {"input": tmp_path, "batch_size": 1}
        if language:
            generate_kwargs["language"] = language

        start = time.time()
        result = ASR_MODEL.generate(**generate_kwargs)
        elapsed = time.time() - start

        text = clean_text(result[0].get("text", ""))
        if response_format == "text":
            return PlainTextResponse(text)

        if response_format == "verbose_json":
            segments = []
            for segment in result[0].get("sentence_info", []):
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
        if tmp_path:
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
        "models_loaded": [MODEL_ID] if ASR_MODEL is not None else [],
    }


def main():
    parser = argparse.ArgumentParser(description="FunASR local model API server")
    parser.add_argument("--host", default="0.0.0.0", help="Bind host")
    parser.add_argument("--port", type=int, default=8000, help="Bind port")
    parser.add_argument("--device", default="cpu", help="Device: cuda, cpu, mps")
    parser.add_argument("--model-path", required=True, help="Local model directory")
    parser.add_argument("--model-id", default="local", help="Model id exposed by the API")
    args, unknown_args = parser.parse_known_args()
    if unknown_args:
        logger.warning("Ignoring unsupported server args: %s", unknown_args)

    global ASR_MODEL, DEVICE, MODEL_ID, MODEL_PATH
    DEVICE = args.device
    MODEL_ID = args.model_id
    MODEL_PATH = args.model_path
    ASR_MODEL = load_local_model(args.model_path, args.device)

    logger.info("FunASR API server starting on http://%s:%s", args.host, args.port)
    uvicorn.run(app, host=args.host, port=args.port)


if __name__ == "__main__":
    main()
