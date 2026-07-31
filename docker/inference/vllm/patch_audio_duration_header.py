"""Patch vLLM v0.24.0 to expose the decoded input audio duration.

The patch is intentionally version-locked. It must fail during the image build
when vLLM changes, rather than silently producing an image without the header.
"""

from pathlib import Path

import vllm


def replace_once(path: Path, old: str, new: str) -> None:
    content = path.read_text()
    if content.count(old) != 1:
        raise RuntimeError(f"unexpected vLLM v0.24.0 source in {path}")
    path.write_text(content.replace(old, new))


vllmRoot = Path(vllm.__file__).resolve().parent
servingPath = vllmRoot / "entrypoints/speech_to_text/base/serving.py"
routerPath = vllmRoot / "entrypoints/speech_to_text/transcription/api_router.py"

replace_once(
    servingPath,
    """        engine_inputs, duration_s = await self._preprocess_speech_to_text(
            request=request,
            audio_data=audio_data,
            request_id=request_id,
        )
""",
    """        engine_inputs, duration_s = await self._preprocess_speech_to_text(
            request=request,
            audio_data=audio_data,
            request_id=request_id,
        )
        raw_request.state.audio_duration_s = duration_s
""",
)

replace_once(
    routerPath,
    """    generator = await handler.create_transcription(audio_data, request, raw_request)

    if isinstance(generator, ErrorResponse):
        return JSONResponse(
            content=generator.model_dump(), status_code=generator.error.code
        )

    elif isinstance(generator, TranscriptionResponseVariant):
        return JSONResponse(content=generator.model_dump())

    return StreamingResponse(content=generator, media_type="text/event-stream")
""",
    """    generator = await handler.create_transcription(audio_data, request, raw_request)
    durationS = getattr(raw_request.state, "audio_duration_s", None)
    headers = (
        {"Audio-Duration-Seconds": f"{durationS:.3f}"}
        if durationS is not None
        else None
    )

    if isinstance(generator, ErrorResponse):
        return JSONResponse(
            content=generator.model_dump(), status_code=generator.error.code
        )

    elif isinstance(generator, TranscriptionResponseVariant):
        return JSONResponse(content=generator.model_dump(), headers=headers)

    return StreamingResponse(
        content=generator,
        media_type="text/event-stream",
        headers=headers,
    )
""",
)
