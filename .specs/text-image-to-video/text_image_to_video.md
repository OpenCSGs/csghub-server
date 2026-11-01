# Text / Image to Video

## Goal

Extend `aigateway` video generation to fully support OpenAI-compatible image-guided video creation on the existing async video APIs:

- `POST /v1/videos`
- `GET /v1/videos/{video_id}`
- `GET /v1/videos/{video_id}/content`

The external API must stay OpenAI-compatible. Image-to-video is not a separate public route; it is the same `POST /v1/videos` request with `input_reference`.

## Web Findings

Current OpenAI documentation and guide show:

- image-guided video uses `POST /v1/videos` with `input_reference`
- `input_reference` can be sent as:
  - multipart uploaded file
  - JSON object with `file_id`
  - JSON object with `image_url`
- OpenAI video uses public `size` values in `{width}x{height}` form
- supported image formats are `image/jpeg`, `image/png`, `image/webp`
- the reference image should match the requested output `size`
- polling should use a reasonable interval such as 10–20 seconds with backoff
- `/content` supports `variant=video|thumbnail|spritesheet`

Reference links:

- https://developers.openai.com/api/docs/guides/video-generation
- https://developers.openai.com/api/reference/resources/videos

Current MiniMax documentation shows:

- MiniMax native create requests use `resolution`, not OpenAI `size`
- supported MiniMax `resolution` values include `720P`, `768P`, and `1080P`

Reference link:

- https://platform.minimaxi.com/docs/api-reference/video-generation-t2v#body-resolution

Current Wan2.2 primary sources show:

- `Wan-AI/Wan2.2-T2V-A14B` is the text-to-video model
- `Wan-AI/Wan2.2-I2V-A14B` is the image-to-video model
- official invocation is documented as CLI / Python usage, not an HTTP API contract
- official parameters include `task`, `prompt`, `size`, `image` for I2V, `frame_num`, and sampling options
- the reference implementation writes `.mp4` output files and does not define HTTP request path, JSON payload, or JSON response
- OpenCSG exposes these Wan2.2 models as model repositories, while LightX2V is the internal video-generation inference runtime used to serve them

Reference links:

- https://github.com/ModelTC/LightX2V
- https://opencsg.com/models/Wan-AI/Wan2.2-T2V-A14B
- https://opencsg.com/models/Wan-AI/Wan2.2-I2V-A14B

## Final Decisions

- Keep the existing public endpoints. Do not add `/v1/image-to-video`.
- Support both backend task labels:
  - `text-to-video`
  - `image-to-video`
- Expose one normalized OpenAI-compatible video API. Provider route, payload, response, status, and download differences must be normalized inside AIGateway adapters.
- Keep OpenAI-compatible providers as the default adapter path. They do not require `metadata.video_api`.
- Select nonstandard provider adapters through simple metadata only:
  - `metadata.video_api.type = "minimax"`
  - `metadata.video_api.type = "seedance"`
- Use adapter capability checks, not only task-name checks, to decide whether a backend can handle:
  - text-only generation
  - image-guided generation
  - multipart `input_reference`
  - JSON `input_reference.file_id`
  - JSON `input_reference.image_url`
- Treat malformed `input_reference` as `invalid_request_error`.
- Reject unsupported uploaded file types locally.
- Preserve `variant` passthrough on `/v1/videos/{video_id}/content`.
- Keep `ai_generations` as the server-side registry for generated AI resources. Current video rows use `resource_type = "video"`. Do not use stateless signed IDs as the primary design.
- Return gateway-owned video IDs to clients. Provider IDs are stored only as routing metadata because provider IDs are not globally unique across backends.
- Do not put provider route maps, request mappings, response mappings, or JSON paths in `llm_config.metadata`.

## Request Contract

### JSON

`POST /v1/videos`

Required:

- `model`
- `prompt`

Optional:

- `size`
- `seconds`
- `input_reference`

`size` is the public OpenAI-compatible field and should use `{width}x{height}` values.

`input_reference` JSON shape:

```json
{
  "file_id": "file_xxx"
}
```

or

```json
{
  "image_url": "https://example.com/frame.png"
}
```

Invalid JSON examples:

- empty object
- object containing neither `file_id` nor `image_url`

### Multipart

Multipart requests continue to use `POST /v1/videos`.

Relevant fields:

- `model`
- `prompt`
- `size`
- `seconds`
- `input_reference` file

Multipart `size` follows the same public OpenAI-compatible `{width}x{height}` contract.

Allowed uploaded content types:

- `image/jpeg`
- `image/png`
- `image/webp`

## Design

### AIGateway Handler

`CreateVideo` should:

1. Parse the request as JSON or multipart.
2. Validate `model` and `prompt`.
3. Validate `input_reference` shape:
   - JSON: require `file_id` or `image_url`
   - multipart: allow at most one uploaded `input_reference` file and verify MIME type
4. Resolve the target model through the existing model lookup path.
5. Select a text-to-video adapter.
6. Ask the adapter for capability flags.
7. Reject requests where the selected backend cannot support the requested input mode.
8. Run existing balance and prompt moderation checks.
9. Forward the normalized request downstream.
10. Persist `gateway video_id -> provider video_id/owner/model/status` in `ai_generations` for follow-up authorization and routing.

`ai_generations` is intentionally the minimal durable registry for this feature. It should be used to:

- hide cross-user access by requiring the stored `owner_uuid` to match the current user
- avoid provider ID collisions by returning a gateway-owned `video_id` to clients and storing the downstream provider ID separately
- route `GET /v1/videos/{video_id}` and `GET /v1/videos/{video_id}/content` back to the model used at creation time
- keep the last observed provider status

Recommended `ai_generations` schema:

- `id`: database primary key
- `resource_type`: generated resource kind, currently `video`
- `resource_id`: gateway-owned public resource ID, currently `video_xxx`
- `provider_resource_id`: downstream provider resource ID
- `provider_metadata`: provider-private metadata such as MiniMax `file_id` or Seedance temporary result URL
- `owner_uuid`: authenticated user UUID that owns the generation
- `model_id`: requested gateway model ID used to resolve the backend for polling/download
- `status`: last observed provider status
- `created_at`
- `updated_at`

Uniqueness should be scoped to `(resource_type, resource_id)`. Do not make `provider_resource_id` globally unique because different providers or backends may produce the same ID.

Create flow ID handling:

- call downstream provider first
- read the provider video ID from the successful response
- generate a gateway video ID
- store both IDs in `ai_generations`
- rewrite the response ID to the gateway video ID before returning to the client

Follow-up flow ID handling:

- look up `ai_generations` by `resource_type = "video"` and the gateway video ID
- require `owner_uuid` to match the current authenticated user
- resolve the backend from `model_id`
- call the downstream provider using `provider_resource_id` and any private `provider_metadata`
- rewrite returned video object IDs back to the gateway video ID

### Adapter Layer

The `text2video` adapter interface should expose capability metadata and own create, retrieve, and content route behavior.

Required capability flags:

- `SupportsCreate`
- `SupportsImageReference`
- `SupportsMultipartInputReference`
- `SupportsJSONFileID`
- `SupportsJSONImageURL`

The OpenAI-compatible adapter should:

- be used when `metadata.video_api` is absent
- match both `text-to-video` and `image-to-video`
- preserve JSON `input_reference` exactly
- allow multipart `input_reference`
- use `llm_config.api_endpoint` for create, append `/{provider_resource_id}` for retrieve, and append `/{provider_resource_id}/content` for content

The MiniMax adapter should:

- be selected by `metadata.video_api.type = "minimax"`
- translate OpenAI-compatible create requests to MiniMax text-to-video or image-to-video payloads
- normalize public OpenAI-compatible `size` values into MiniMax `resolution` values
  - `1280x720` and `720x1280` -> `720P`
  - `1920x1080` and `1080x1920` -> `1080P`
  - accept native MiniMax `720P`, `768P`, and `1080P` for compatibility
  - reject unsupported sizes such as `1024x1792` as `invalid_request_error`
- call MiniMax query API for retrieve using the provider task ID
- store MiniMax `file_id` in `ai_generations.provider_metadata`
- call MiniMax file retrieve/download flow for content
- normalize MiniMax statuses into OpenAI-compatible statuses

The Seedance adapter should:

- be selected by `metadata.video_api.type = "seedance"`
- translate OpenAI-compatible create requests to Seedance/BytePlus content generation task payloads
- call Seedance task query API for retrieve
- store private result URL metadata when present
- stream generated video URLs through AIGateway for content
- normalize Seedance statuses into OpenAI-compatible statuses

`llm_config.metadata` examples:

OpenAI-compatible:

```json
{
  "tasks": ["text-to-video", "image-to-video"]
}
```

MiniMax:

```json
{
  "tasks": ["text-to-video", "image-to-video"],
  "video_api": {
    "type": "minimax"
  }
}
```

Seedance:

```json
{
  "tasks": ["text-to-video", "image-to-video"],
  "video_api": {
    "type": "seedance"
  }
}
```

Unsupported `metadata.video_api.type` should return a clear configuration error instead of silently falling back to OpenAI-compatible behavior.

The internal CSGHub adapter should:

- exist separately from the OpenAI-compatible adapter because the Wan2.2 official models do not expose an OpenAI-compatible HTTP contract
- target the internal LightX2V runtime that serves the OpenCSG Wan model repos, not the model repos themselves
- translate OpenAI `POST /v1/videos` requests into the internal CSGHub video serving API
- support at least:
  - text-only video for OpenCSG model `Wan-AI/Wan2.2-T2V-A14B`
  - image-guided video for OpenCSG model `Wan-AI/Wan2.2-I2V-A14B`
- use runtime framework selection for the internal LightX2V deployment instead of `metadata.video_api`
- require internal CSGHub deployment identity, not runtime framework alone
  - `CSGHubModelID != ""`
  - `RuntimeFramework == "lightx2v"`
- map OpenAI request fields onto internal backend fields such as:
  - `prompt`
  - `size`
  - `seconds` or frame count / duration equivalent
  - reference image input for I2V
- normalize internal backend responses back into the OpenAI-compatible async video object used by `aigateway`

Because the official Wan sources do not define HTTP path / payload / response, the internal CSGHub serving contract must be treated as StarHub-owned API design. The adapter should therefore target the actual internal runner / model serving endpoint used in CSGHub, not a guessed “official Wan HTTP API”.

Recommended backend mapping:

- text-only requests:
  - route to internal LightX2V runtime serving OpenCSG model `Wan-AI/Wan2.2-T2V-A14B`
- image-guided requests:
  - route to internal LightX2V runtime serving OpenCSG model `Wan-AI/Wan2.2-I2V-A14B`
  - support multipart uploaded images
  - support JSON `input_reference.image_url` by fetching the image and converting it into the backend-native multipart upload
  - reject JSON `input_reference.file_id` until AIGateway has a gateway-side file resolution flow for video inputs

Do not force image-guided requests into `Wan2.2-T2V-A14B`, because the official model split indicates I2V support belongs to the dedicated `I2V-A14B` line.

### Internal CSGHub Contract

The internal adapter must support backend-specific path and payload translation.

Required design rule:

- keep the external API OpenAI-compatible
- allow internal request path, request body, and response body to differ
- keep all such translation inside the CSGHub adapter, not in the generic handler

The exact internal request contract should be discovered from the deployed CSGHub video serving implementation. At minimum, the adapter must be able to translate:

- create request
- status polling request
- content download request

and map those into:

- `POST /v1/videos`
- `GET /v1/videos/{video_id}`
- `GET /v1/videos/{video_id}/content`

Implemented LightX2V contract:

- create JSON T2V: `POST /v1/tasks/video`
- create multipart I2V: `POST /v1/tasks/video/form`
- create multipart T2V without uploaded `input_reference`: fallback to JSON T2V `POST /v1/tasks/video`
- status poll: `GET /v1/tasks/{task_id}/status`
- content download: `GET /v1/files/download/outputs/videos/{task_id}.mp4`

### Follow-up Endpoints

`GetVideo` remains unchanged except for task matching through the broader adapter capability rules.

`GetVideoContent` should continue proxying the downstream response and must preserve query passthrough for:

- `variant=video`
- `variant=thumbnail`
- `variant=spritesheet`

## Validation Rules

- `input_reference` JSON object must include `file_id` or `image_url`
- only one multipart `input_reference` file is accepted
- multipart file type must be `jpeg/png/webp`
- if the selected adapter lacks image-guided support, return `invalid_request_error`
- if the selected adapter lacks `file_id` or `image_url` support, return `invalid_request_error`
- for LightX2V, `input_reference.image_url` requires gateway-side remote image fetch and therefore must be treated as untrusted remote input

The handler may keep image-dimension validation lightweight. If local image size inspection is not cheap or robust enough, the gateway can pass the request through and let the downstream provider enforce exact `size` matching.

## Out of Scope

- new public image-to-video endpoint
- list/delete/webhook additions for generated video resources
- persistent storage of generated video blobs inside `aigateway`
