# AIGateway Video API

## Overview

AIGateway exposes one normalized OpenAI-compatible async video API surface for both text-to-video and image-to-video generation.

Public endpoints:

```text
POST /v1/videos
GET /v1/videos/{video_id}
GET /v1/videos/{video_id}/content
```

The public API does not expose provider-specific paths, IDs, request fields, or download URLs. AIGateway normalizes those differences internally.

For an end-to-end usage walkthrough, see [user-guide.md](./user-guide.md).

## Auth

Video APIs require the normal AIGateway API key auth used by other OpenAI-compatible inference APIs:

```http
Authorization: Bearer <api_key_or_access_token>
```

## Resource Model

The client-facing `video_id` is owned by AIGateway, not by the downstream provider.

Example:

```text
video_8b1d8d4f7c5b4d58b4d7f3a7f8c9d001
```

This lets AIGateway:

- authorize follow-up reads by owner
- avoid cross-provider ID collisions
- route retrieve and content requests back to the backend used at create time

## Create Video

```http
POST /v1/videos
```

Supports:

- text-to-video
- image-to-video by `input_reference`

### JSON Request

Required fields:

- `model`
- `prompt`

Optional fields:

- `size`
- `seconds`
- `input_reference`
- additional OpenAI-compatible fields supported by the downstream OpenAI-compatible backend

`size` follows the public OpenAI-compatible contract and should use `{width}x{height}` values such as `1280x720` or `1920x1080`.
Provider-native adapters may normalize that value internally. For example, MiniMax uses a native `resolution` field and AIGateway maps supported values like `1280x720 -> 720P` and `1920x1080 -> 1080P`.

JSON request shape:

```json
{
  "model": "video-model",
  "prompt": "A paper airplane flying through a neon city",
  "size": "1280x720",
  "seconds": 5,
  "input_reference": {
    "image_url": "https://example.com/frame.png"
  }
}
```

`input_reference` supports:

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

Provider-native adapters may support only part of the public `input_reference` surface. For example, the internal LightX2V adapter supports multipart uploaded images and JSON `image_url`, but rejects JSON `file_id`.

Invalid:

- missing `model`
- missing `prompt`
- empty `input_reference`
- `input_reference` with neither `file_id` nor `image_url`

### Multipart Request

`POST /v1/videos` also accepts multipart form data.

Relevant fields:

- `model`
- `prompt`
- `size`
- `seconds`
- `input_reference` file

Multipart `size` uses the same OpenAI-compatible `{width}x{height}` contract as JSON requests.

Multipart text-only requests remain valid. Provider-native adapters may normalize them into backend-specific JSON create requests when no uploaded `input_reference` file is present.

Allowed uploaded `input_reference` content types:

- `image/jpeg`
- `image/png`
- `image/webp`

Multipart example:

```bash
curl -X POST "$AIGATEWAY_BASE_URL/v1/videos" \
  -H "Authorization: Bearer $AIGATEWAY_API_KEY" \
  -F "model=image-video-model" \
  -F "prompt=Animate this still frame into a cinematic shot" \
  -F "seconds=5" \
  -F "size=1280x720" \
  -F "input_reference=@frame.png;type=image/png"
```

### Successful Response

```json
{
  "id": "video_8b1d8d4f7c5b4d58b4d7f3a7f8c9d001",
  "object": "video",
  "status": "queued"
}
```

Status values returned by AIGateway are normalized to OpenAI-compatible values such as:

- `queued`
- `in_progress`
- `completed`
- `failed`
- `cancelled`

### Error Response

Create errors use the normal OpenAI-compatible error envelope:

```json
{
  "error": {
    "code": "invalid_request_error",
    "message": "Model and prompt cannot be empty",
    "type": "invalid_request_error"
  }
}
```

Common create error codes:

- `invalid_request_error`
- `unsupported_model`
- `content_policy_violation`
- `moderation_error`
- `internal_error`

Example provider-specific validation error:

```json
{
  "error": {
    "code": "invalid_request_error",
    "message": "selected model does not support input_reference.file_id",
    "type": "invalid_request_error"
  }
}
```

## Get Video

```http
GET /v1/videos/{video_id}
```

Returns the normalized video object for a gateway-owned `video_id`.

Example:

```json
{
  "id": "video_8b1d8d4f7c5b4d58b4d7f3a7f8c9d001",
  "object": "video",
  "status": "completed",
  "created_at": 1713945600
}
```

If the video failed, the object may include an embedded resource-level error:

```json
{
  "id": "video_8b1d8d4f7c5b4d58b4d7f3a7f8c9d001",
  "object": "video",
  "status": "failed",
  "error": {
    "code": "generation_failed",
    "message": "provider generation failed"
  }
}
```

HTTP-level lookup errors still use the normal top-level `error` envelope.

## Download Video Content

```http
GET /v1/videos/{video_id}/content
```

This endpoint streams the generated video bytes back through AIGateway.

Typical content types:

- `video/mp4`
- `application/octet-stream`

Example:

```bash
curl -L "$AIGATEWAY_BASE_URL/v1/videos/$VIDEO_ID/content" \
  -H "Authorization: Bearer $AIGATEWAY_API_KEY" \
  -o output.mp4
```

### Variant Passthrough

AIGateway preserves the `variant` query parameter for compatible backends.

Examples:

```text
GET /v1/videos/{video_id}/content?variant=video
GET /v1/videos/{video_id}/content?variant=thumbnail
GET /v1/videos/{video_id}/content?variant=spritesheet
```

Whether a backend supports a specific variant depends on the downstream provider. Unsupported variants are surfaced as downstream/provider errors.

## Ownership and Authorization

Video resources are private to the authenticated owner that created them.

For follow-up operations:

- `GET /v1/videos/{video_id}`
- `GET /v1/videos/{video_id}/content`

AIGateway verifies that the current user owns the `video_id`. Cross-user access returns `not_found`.

## Provider Normalization Notes

The external API is stable even when provider APIs differ internally.

Examples of internal normalization:

- OpenAI-compatible backends accept the public `size` field directly in `{width}x{height}` format.
- MiniMax does not use the same public shape. AIGateway maps supported OpenAI-compatible sizes to MiniMax `resolution` values:
  - `1280x720` and `720x1280` -> `720P`
  - `1920x1080` and `1080x1920` -> `1080P`
  - native MiniMax values `720P`, `768P`, and `1080P` are also accepted for compatibility
  - unsupported sizes such as `1024x1792` are rejected as `invalid_request_error`
- Internal LightX2V uses provider-native create and status routes. AIGateway maps public OpenAI-compatible `size` into LightX2V `width` and `height`, streams `/content` directly from `/v1/files/download/outputs/videos/{task_id}.mp4`, and supports image-guided requests only through multipart upload or JSON `image_url`.
- Internal LightX2V is selected only for internal CSGHub deployed video models, identified by `CSGHubModelID != ""` plus `RuntimeFramework == "lightx2v"`.
- Internal LightX2V treats multipart-with-image as image-guided create to `/v1/tasks/video/form`, but multipart text-only requests fall back to the normal JSON create path `/v1/tasks/video`.
- Current internal LightX2V-backed OpenCSG Wan model targets are `Wan-AI/Wan2.2-T2V-A14B` for text-to-video and `Wan-AI/Wan2.2-I2V-A14B` for image-to-video.

- AIGateway rewrites provider video IDs to gateway-owned `video_id`
- provider-specific polling APIs are hidden behind `GET /v1/videos/{video_id}`
- provider-specific file download or download URL flows are hidden behind `GET /v1/videos/{video_id}/content`
- provider-native status values are normalized to OpenAI-compatible values

## Current Public DTO Shape

Create request:

```json
{
  "model": "string",
  "prompt": "string",
  "size": "string",
  "seconds": 5,
  "input_reference": {
    "file_id": "string",
    "image_url": "string"
  }
}
```

Video object:

```json
{
  "id": "string",
  "object": "video",
  "created_at": 0,
  "completed_at": 0,
  "expires_at": 0,
  "status": "queued",
  "model": "string",
  "prompt": "string",
  "size": "string",
  "seconds": 0,
  "progress": 0.5,
  "error": {
    "code": "string",
    "message": "string"
  },
  "remixed_from_video_id": "string"
}
```
