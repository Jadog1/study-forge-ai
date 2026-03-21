import os
import json
from threading import Thread
from typing import Union

import torch
import uvicorn
from fastapi import FastAPI
from pydantic import BaseModel
from fastapi.responses import StreamingResponse
from transformers import AutoModelForCausalLM, AutoTokenizer, TextIteratorStreamer

try:
    from transformers import BitsAndBytesConfig
except ImportError:
    BitsAndBytesConfig = None


# Qwen/Qwen2.5-3B-Instruct
# mistralai/Mistral-7B-Instruct-v0.2
MODEL_ID = os.getenv("MODEL_ID", "mistralai/Mistral-7B-Instruct-v0.2")
HOST = os.getenv("HOST", "127.0.0.1")
PORT = int(os.getenv("PORT", "8000"))

app = FastAPI()


def load_model_and_tokenizer():
    print(f"Loading model: {MODEL_ID}")
    tokenizer = AutoTokenizer.from_pretrained(MODEL_ID)

    model_kwargs = {"device_map": "auto"}
    if torch.cuda.is_available():
        model_kwargs["dtype"] = torch.float16
        if BitsAndBytesConfig is not None:
            model_kwargs["quantization_config"] = BitsAndBytesConfig(load_in_4bit=True)
        else:
            print("bitsandbytes is unavailable; loading without 4-bit quantization.")
    else:
        model_kwargs["dtype"] = torch.float32
        print("CUDA not detected; loading without 4-bit quantization.")

    try:
        model = AutoModelForCausalLM.from_pretrained(MODEL_ID, **model_kwargs)
    except (ImportError, RuntimeError, TypeError, ValueError) as exc:
        if "quantization_config" not in model_kwargs:
            raise

        print(f"4-bit load failed ({exc}); retrying without quantization.")
        fallback_kwargs = {key: value for key, value in model_kwargs.items() if key != "quantization_config"}
        model = AutoModelForCausalLM.from_pretrained(MODEL_ID, **fallback_kwargs)

    print("Model loaded!")
    return tokenizer, model


tokenizer, model = load_model_and_tokenizer()

# ---- Request Schema ----
class Message(BaseModel):
    role: str
    content: str

class ChatRequest(BaseModel):
    model: str
    messages: list[Message]
    max_tokens: int = 10000
    temperature: float = 0.7
    stream: bool = False


class EmbeddingRequest(BaseModel):
    model: str
    input: Union[str, list[str]]

# ---- Helper ----
def format_prompt(messages):
    # Simple chat formatting (works well with Mistral instruct)
    prompt = ""
    for m in messages:
        if m.role == "system":
            prompt += f"<s>[INST] {m.content} [/INST]"
        elif m.role == "user":
            prompt += f"<s>[INST] {m.content} [/INST]"
        elif m.role == "assistant":
            prompt += m.content
    return prompt


def build_inputs_and_kwargs(req: ChatRequest):
    prompt = format_prompt(req.messages)
    inputs = tokenizer(prompt, return_tensors="pt").to(model.device)
    generate_kwargs = {
        **inputs,
        "max_new_tokens": req.max_tokens,
        "temperature": req.temperature,
        "do_sample": req.temperature > 0,
    }
    return prompt, inputs, generate_kwargs


def completion_response(content: str, prompt_tokens: int, completion_tokens: int):
    return {
        "id": "chatcmpl-local",
        "object": "chat.completion",
        "choices": [
            {
                "index": 0,
                "message": {
                    "role": "assistant",
                    "content": content,
                },
                "finish_reason": "stop",
            }
        ],
        "usage": {
            "prompt_tokens": prompt_tokens,
            "completion_tokens": completion_tokens,
            "total_tokens": prompt_tokens + completion_tokens,
        },
    }


def embedding_response(model_name: str, embeddings: list[list[float]], prompt_tokens: int):
    return {
        "object": "list",
        "model": model_name,
        "data": [
            {
                "object": "embedding",
                "index": i,
                "embedding": emb,
            }
            for i, emb in enumerate(embeddings)
        ],
        "usage": {
            "prompt_tokens": prompt_tokens,
            "total_tokens": prompt_tokens,
        },
    }


def compute_embeddings(texts: list[str]):
    vectors = []
    token_count = 0
    embed_layer = model.get_input_embeddings()

    for text in texts:
        inputs = tokenizer(
            text,
            return_tensors="pt",
            truncation=True,
            max_length=2048,
        )
        input_ids = inputs["input_ids"].to(model.device)
        token_count += int(input_ids.numel())

        with torch.no_grad():
            token_embeds = embed_layer(input_ids)
            pooled = token_embeds.mean(dim=1).squeeze(0)

        vectors.append(pooled.detach().float().cpu().tolist())

    return vectors, token_count


def stream_completion(req: ChatRequest):
    _, inputs, generate_kwargs = build_inputs_and_kwargs(req)
    streamer = TextIteratorStreamer(tokenizer, skip_prompt=True, skip_special_tokens=True)
    generation_error = []
    generated_parts = []
    prompt_tokens = int(inputs["input_ids"].shape[1])

    def run_generation():
        try:
            with torch.no_grad():
                model.generate(**generate_kwargs, streamer=streamer)
        except Exception as exc:  # pragma: no cover - surfaced through the stream
            generation_error.append(exc)
            streamer.end()

    thread = Thread(target=run_generation, daemon=True)
    thread.start()

    def event_stream():
        try:
            for text in streamer:
                if not text:
                    continue
                generated_parts.append(text)
                chunk = {
                    "id": "chatcmpl-local",
                    "object": "chat.completion.chunk",
                    "choices": [
                        {
                            "index": 0,
                            "delta": {"content": text},
                            "finish_reason": None,
                        }
                    ],
                }
                yield f"data: {json.dumps(chunk)}\n\n"

            thread.join()
            if generation_error:
                error_chunk = {
                    "error": {"message": str(generation_error[0])},
                }
                yield f"data: {json.dumps(error_chunk)}\n\n"
                yield "data: [DONE]\n\n"
                return

            completion_tokens = len(tokenizer("".join(generated_parts), add_special_tokens=False)["input_ids"])
            final_chunk = {
                "id": "chatcmpl-local",
                "object": "chat.completion.chunk",
                "choices": [
                    {
                        "index": 0,
                        "delta": {},
                        "finish_reason": "stop",
                    }
                ],
                "usage": {
                    "prompt_tokens": prompt_tokens,
                    "completion_tokens": completion_tokens,
                    "total_tokens": prompt_tokens + completion_tokens,
                },
            }
            yield f"data: {json.dumps(final_chunk)}\n\n"
            yield "data: [DONE]\n\n"
        finally:
            if thread.is_alive():
                thread.join(timeout=1)

    headers = {
        "Cache-Control": "no-cache",
        "Connection": "keep-alive",
        "X-Accel-Buffering": "no",
    }
    return StreamingResponse(event_stream(), media_type="text/event-stream", headers=headers)

# ---- Endpoint ----
@app.post("/v1/chat/completions")
def chat(req: ChatRequest):
    if req.stream:
        return stream_completion(req)

    prompt, inputs, generate_kwargs = build_inputs_and_kwargs(req)

    with torch.no_grad():
        outputs = model.generate(**generate_kwargs)

    prompt_tokens = int(inputs["input_ids"].shape[1])
    generated_tokens = outputs[0][inputs["input_ids"].shape[1]:]
    completion_tokens = int(generated_tokens.shape[0])
    response_text = tokenizer.decode(generated_tokens, skip_special_tokens=True)

    return completion_response(response_text, prompt_tokens, completion_tokens)


@app.post("/v1/embeddings")
def embeddings(req: EmbeddingRequest):
    inputs = req.input if isinstance(req.input, list) else [req.input]
    if not inputs:
        return {
            "error": {"message": "input must contain at least one string"},
        }

    vectors, token_count = compute_embeddings(inputs)
    return embedding_response(req.model, vectors, token_count)


if __name__ == "__main__":
    uvicorn.run(app, host=HOST, port=PORT)