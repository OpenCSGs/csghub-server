import os
import sys
import logging
import asyncio
sys.path.insert(0, '/sgl-workspace/sglang/python')
import torch
import torch.nn.functional as F
import json
import uuid
import time
from fastapi import FastAPI, HTTPException, Request
from pydantic import BaseModel
from typing import List, Optional
import uvicorn
from transformers import AutoTokenizer
from sglang.srt.entrypoints.engine import Engine

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] %(message)s',
    force=True
)
logger = logging.getLogger('qwen-guard')
logger.setLevel(logging.INFO)

repo_id = os.getenv('REPO_ID', 'qwen3-guard')
MODEL_PATH = os.getenv('MODEL_PATH', f'/workspace/{repo_id}')
CONTEXT_LENGTH = int(os.getenv('CONTEXT_LENGTH', '8192'))
TP_SIZE = int(os.getenv('TP_SIZE', '1'))
MEM_FRACTION_STATIC = float(os.getenv('MEM_FRACTION_STATIC', '0.6'))
PAGE_SIZE = int(os.getenv('PAGE_SIZE', '1'))
CHUNKED_PREFILL_SIZE = int(os.getenv('CHUNKED_PREFILL_SIZE', '131072'))
PORT = int(os.getenv('PORT', '8000'))
WARMUP_ENABLED = os.getenv('WARMUP_ENABLED', 'true').strip().lower() in {'1', 'true', 'yes', 'y', 'on'}
WARMUP_DELAY_SECONDS = int(os.getenv('WARMUP_DELAY_SECONDS', '60'))
WARMUP_PROMPT = os.getenv('WARMUP_PROMPT', 'hello')

logger.info(f'MODEL_PATH: {MODEL_PATH}')
logger.info(f'CONTEXT_LENGTH: {CONTEXT_LENGTH}')
logger.info(f'TP_SIZE: {TP_SIZE}')
logger.info(f'MEM_FRACTION_STATIC: {MEM_FRACTION_STATIC}')
logger.info(f'PAGE_SIZE: {PAGE_SIZE}')
logger.info(f'CHUNKED_PREFILL_SIZE: {CHUNKED_PREFILL_SIZE}')
logger.info(f'PORT: {PORT}')
logger.info(f'WARMUP_ENABLED: {WARMUP_ENABLED}')
logger.info(f'WARMUP_DELAY_SECONDS: {WARMUP_DELAY_SECONDS}')

logger.info('Loading tokenizer...')
tokenizer = AutoTokenizer.from_pretrained(MODEL_PATH, trust_remote_code=True)
logger.info('Tokenizer loaded')

risk_level_map = {0: 'Safe', 1: 'Unsafe', 2: 'Controversial'}
query_category_map = {0: 'Violent', 1: 'Sexual Content', 2: 'Self-Harm', 3: 'Political', 4: 'PII', 5: 'Copyright', 6: 'Illegal Acts', 7: 'Unethical', 8: 'Jailbreak'}
response_category_map = {0: 'Violent', 1: 'Sexual Content', 2: 'Self-Harm', 3: 'Political', 4: 'PII', 5: 'Copyright', 6: 'Illegal Acts', 7: 'Unethical'}

engine = None

app = FastAPI(title='Qwen3Guard-Stream API')


async def delayed_warmup():
    if not WARMUP_ENABLED:
        return
    if engine is None:
        logger.warning('Warmup skipped: engine is not initialized')
        return

    logger.info(f'Warmup scheduled, will run after {WARMUP_DELAY_SECONDS}s')
    await asyncio.sleep(WARMUP_DELAY_SECONDS)

    warmup_trace_id = f'warmup-{uuid.uuid4().hex[:8]}'
    warmup_rid = f'warmup-{uuid.uuid4().hex}'
    try:
        start = time.time()
        with torch.inference_mode():
            outputs = await engine.async_generate(
                WARMUP_PROMPT,
                sampling_params={'max_new_tokens': 1},
                rid=warmup_rid,
                resumable=False
            )
        infer_time = time.time() - start
        token_stats = get_token_stats(outputs, prompt=WARMUP_PROMPT, infer_cost=infer_time)
        logger.info(
            f'[TraceID: {warmup_trace_id}] Warmup done'
            f' | PromptTokens: {token_stats["prompt_tokens"]}'
            f' | CompletionTokens: {token_stats["completion_tokens"]}'
            f' | Infer: {infer_time*1000:.1f}ms'
            f' | TPM(total): {token_stats["total_tpm"]:.2f}'
        )
    except Exception:
        logger.exception(f'[TraceID: {warmup_trace_id}] Warmup failed')

class ChatMessage(BaseModel):
    role: str
    content: str

class ChatCompletionRequest(BaseModel):
    model: str
    messages: List[ChatMessage]
    stream: Optional[bool] = False


def parse_bool_header(value: Optional[str]) -> Optional[bool]:
    if not isinstance(value, str):
        return None
    normalized = value.strip().lower()
    if normalized in {'1', 'true', 'yes', 'y', 'on'}:
        return True
    if normalized in {'0', 'false', 'no', 'n', 'off'}:
        return False
    return None


def get_token_stats(result, prompt: str, infer_cost: float):
    prompt_tokens = 0
    completion_tokens = 1  # Guard classification typically returns one token.

    meta = None
    if isinstance(result, dict):
        meta = result.get('meta_info')
    elif hasattr(result, 'meta_info'):
        meta = getattr(result, 'meta_info')

    if isinstance(meta, dict):
        prompt_tokens = int(meta.get('prompt_tokens', 0) or 0)
        completion_tokens = int(meta.get('completion_tokens', 1) or 1)
    elif meta is not None:
        prompt_tokens = int(getattr(meta, 'prompt_tokens', 0) or 0)
        completion_tokens = int(getattr(meta, 'completion_tokens', 1) or 1)

    # Fallback to an estimation for logging if engine meta does not provide token counts.
    if prompt_tokens <= 0 and prompt:
        prompt_tokens = max(len(prompt) // 2, 1)

    total_tokens = prompt_tokens + completion_tokens
    generation_tps = completion_tokens / infer_cost if infer_cost > 0 else 0.0
    total_tps = total_tokens / infer_cost if infer_cost > 0 else 0.0

    # Tokens per minute estimation.
    generation_tpm = generation_tps * 60
    total_tpm = total_tps * 60

    return {
        'prompt_tokens': prompt_tokens,
        'completion_tokens': completion_tokens,
        'total_tokens': total_tokens,
        'generation_tps': generation_tps,
        'total_tps': total_tps,
        'generation_tpm': generation_tpm,
        'total_tpm': total_tpm
    }

def process_result(result, type_='query'):
    if type_ == 'query':
        risk_logits = torch.tensor(result['query_risk_level_logits']).view(-1, 3)
        category_logits = torch.tensor(result['query_category_logits']).view(-1, 9)
    else:
        risk_logits = torch.tensor(result['risk_level_logits']).view(-1, 3)
        category_logits = torch.tensor(result['category_logits']).view(-1, 8)
    pred_risk = torch.argmax(risk_logits, dim=1).tolist()[-1]
    pred_category = torch.argmax(category_logits, dim=1).tolist()[-1]
    if type_ == 'query':
        return {'risk_level': risk_level_map[pred_risk], 'category': query_category_map[pred_category]}
    return {'risk_level': risk_level_map[pred_risk], 'category': response_category_map[pred_category]}

@app.get('/health')
def health():
    return {'status': 'ok'}

@app.get('/v1/models')
def list_models():
    return {'object': 'list', 'data': [{'id': os.getenv('REPO_ID', 'qwen3-guard'), 'object': 'model', 'owned_by': 'qwen'}]}


@app.on_event('startup')
async def startup_event():
    asyncio.create_task(delayed_warmup())


@app.post('/v1/chat/completions')
async def chat_completions(raw_request: Request, request: ChatCompletionRequest):
    start_time = time.time()
    trace_id = (
        raw_request.headers.get('x-request-id')
        or raw_request.headers.get('request-id')
        or uuid.uuid4().hex
    )

    if not request.messages:
        raise HTTPException(status_code=400, detail='messages cannot be empty')

    last_msg = request.messages[-1]
    prompt = last_msg.content
    sampling_params = {'max_new_tokens': 1}

    rid = (
        raw_request.headers.get('x-session-id')
        or raw_request.headers.get('session-id')
    )
    resumable = parse_bool_header(
        raw_request.headers.get('x-resumable')
        or raw_request.headers.get('resumable')
    )
    if not rid:
        rid = uuid.uuid4().hex
        resumable = False
    if resumable is None:
        resumable = False

    content_for_log = prompt[:100] + '...' if len(prompt) > 100 else prompt
    logger.info(
        f'[TraceID: {trace_id}] >>> Request: role={last_msg.role}, '
        f'content={content_for_log}, rid={rid}, resumable={resumable}'
    )

    try:
        infer_start = time.time()
        with torch.inference_mode():
            outputs = await engine.async_generate(
                prompt,
                sampling_params=sampling_params,
                rid=rid,
                resumable=resumable
            )
        infer_time = time.time() - infer_start

        type_ = 'query' if last_msg.role == 'user' else 'response'
        eval_result = process_result(outputs, type_=type_)
        token_stats = get_token_stats(outputs, prompt=prompt, infer_cost=infer_time)
        total_time = time.time() - start_time

        logger.info(
            f'[TraceID: {trace_id}] <<< Response: {json.dumps(eval_result, ensure_ascii=False)}'
            f' | Mode: {type_}'
            f' | Infer: {infer_time*1000:.1f}ms'
            f' | Total: {total_time*1000:.1f}ms'
            f' | PromptTokens: {token_stats["prompt_tokens"]}'
            f' | CompletionTokens: {token_stats["completion_tokens"]}'
            f' | TPS(gen): {token_stats["generation_tps"]:.2f}'
            f' | TPS(total): {token_stats["total_tps"]:.2f}'
            f' | TPM(gen): {token_stats["generation_tpm"]:.2f}'
            f' | TPM(total): {token_stats["total_tpm"]:.2f}'
        )

        return {
            'id': f'chatcmpl-{rid}',
            'object': 'chat.completion',
            'created': int(time.time()),
            'model': request.model,
            'choices': [{'index': 0, 'message': {'role': 'assistant', 'content': json.dumps(eval_result, ensure_ascii=False)}, 'finish_reason': 'stop'}],
            'usage': {
                'prompt_tokens': token_stats['prompt_tokens'],
                'completion_tokens': token_stats['completion_tokens'],
                'total_tokens': token_stats['total_tokens']
            }
        }
    except torch.cuda.OutOfMemoryError:
        logger.error(f'[TraceID: {trace_id}] CUDA OOM')
        raise HTTPException(status_code=500, detail='GPU out of memory')
    except Exception as e:
        logger.exception(f'[TraceID: {trace_id}] Inference failed')
        raise HTTPException(status_code=500, detail=str(e))

if __name__ == '__main__':
    logger.info('Loading SGLang engine...')
    engine = Engine(
        model_path=MODEL_PATH,
        context_length=CONTEXT_LENGTH,
        page_size=PAGE_SIZE,
        tp_size=TP_SIZE,
        mem_fraction_static=MEM_FRACTION_STATIC,
        chunked_prefill_size=CHUNKED_PREFILL_SIZE,
    )
    logger.info('Engine initialized successfully!')
    logger.info(f'Server starting on http://0.0.0.0:{PORT}')

    uvicorn.run(app, host='0.0.0.0', port=PORT, log_level='info')
