package types

import "context"

// Storage is a generic interface for storing generated content (image, audio, video)
// and returning a presigned GET URL. Same interface for all generated media.
type Storage interface {
	PutAndPresignGet(ctx context.Context, bucket, key string, data []byte, contentType string) (presignedGetURL string, err error)
}

// TransformResponseOptions is passed to T2IAdapter.TransformResponse when the caller
// wants to influence the response (e.g. return URL instead of b64 when response_format=url).
type TransformResponseOptions struct {
	ResponseFormat string // "url" or "b64_json" from the image generation request
	Storage        Storage
	Bucket         string // bucket for upload when using Storage (e.g. from config)
}
