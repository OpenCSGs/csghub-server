import os
import sys
import logging
sys.path.insert(0, '/sgl-workspace/sglang/python')
import torch
import torch.nn.functional as F
import json
import uuid
import time
from fastapi import FastAPI, HTTPException
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

logger.info(f'MODEL_PATH: {MODEL_PATH}')
logger.info(f'CONTEXT_LENGTH: {CONTEXT_LENGTH}')
logger.info(f'TP_SIZE: {TP_SIZE}')
logger.info(f'MEM_FRACTION_STATIC: {MEM_FRACTION_STATIC}')
logger.info(f'PAGE_SIZE: {PAGE_SIZE}')
logger.info(f'CHUNKED_PREFILL_SIZE: {CHUNKED_PREFILL_SIZE}')
logger.info(f'PORT: {PORT}')

logger.info('Loading tokenizer...')
tokenizer = AutoTokenizer.from_pretrained(MODEL_PATH, trust_remote_code=True)
logger.info('Tokenizer loaded')

im_start_id = tokenizer.convert_tokens_to_ids('<|im_start|>')
user_id = tokenizer.convert_tokens_to_ids('user')
im_end_id = tokenizer.convert_tokens_to_ids('<|im_end|>')

risk_level_map = {0: 'Safe', 1: 'Unsafe', 2: 'Controversial'}
query_category_map = {0: 'Violent', 1: 'Sexual Content', 2: 'Self-Harm', 3: 'Political', 4: 'PII', 5: 'Copyright', 6: 'Illegal Acts', 7: 'Unethical', 8: 'Jailbreak'}
response_category_map = {0: 'Violent', 1: 'Sexual Content', 2: 'Self-Harm', 3: 'Political', 4: 'PII', 5: 'Copyright', 6: 'Illegal Acts', 7: 'Unethical'}

engine = None

app = FastAPI(title='Qwen3Guard-Stream API')

class ChatMessage(BaseModel):
    role: str
    content: str

class ChatCompletionRequest(BaseModel):
    model: str
    messages: List[ChatMessage]
    stream: Optional[bool] = False

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

@app.post('/v1/chat/completions')
async def chat_completions(request: ChatCompletionRequest):
    request_id = uuid.uuid4().hex[:8]
    start_time = time.time()

    if not request.messages:
        raise HTTPException(status_code=400, detail='messages cannot be empty')

    last_msg = request.messages[-1]
    logger.info(f'[{request_id}] >>> Request: role={last_msg.role}, content={last_msg.content[:100]}...' if len(last_msg.content) > 100 else f'[{request_id}] >>> Request: role={last_msg.role}, content={last_msg.content}')

    conversation = [{'role': m.role, 'content': m.content} for m in request.messages]
    prompt_text = tokenizer.apply_chat_template(conversation, tokenize=False, add_generation_prompt=True)
    input_ids = tokenizer(prompt_text, return_tensors='pt').input_ids[0].tolist()

    logger.info(f'[{request_id}] Input tokens: {len(input_ids)}')

    last_start = next((i for i in range(len(input_ids)-1, -1, -1) if input_ids[i:i+2] == [im_start_id, user_id]), None)
    user_end_index = next((i for i in range(last_start+2, len(input_ids)) if input_ids[i] == im_end_id), None) if last_start else None

    rid = uuid.uuid4().hex
    last_role = request.messages[-1].role

    infer_start = time.time()
    if last_role == 'user':
        type_ = 'query'
        query_prompt = input_ids[:user_end_index+1] if user_end_index else input_ids
        outputs = await engine.async_generate(input_ids=query_prompt, sampling_params={'max_new_tokens': 1}, rid=rid, resumable=False)
    else:
        type_ = 'response'
        outputs = await engine.async_generate(input_ids=input_ids, sampling_params={'max_new_tokens': 1}, rid=rid, resumable=False)
    infer_time = time.time() - infer_start

    eval_result = process_result(outputs, type_=type_)
    total_time = time.time() - start_time

    logger.info(f'[{request_id}] <<< Response: {json.dumps(eval_result, ensure_ascii=False)} | Infer: {infer_time*1000:.1f}ms | Total: {total_time*1000:.1f}ms')

    return {
        'id': f'chatcmpl-{rid}',
        'object': 'chat.completion',
        'created': int(time.time()),
        'model': request.model,
        'choices': [{'index': 0, 'message': {'role': 'assistant', 'content': json.dumps(eval_result, ensure_ascii=False)}, 'finish_reason': 'stop'}],
        'usage': {'prompt_tokens': len(input_ids), 'completion_tokens': 1, 'total_tokens': len(input_ids) + 1}
    }

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
