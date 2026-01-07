package types

import (
	"io"
	"time"
)

// AccessMode represents the access mode for object storage operations
type AccessMode string

const (
	// ModeDirect allows direct access to OSS without gateway
	ModeDirect AccessMode = "direct"
	// ModeSigned generates presigned URLs for temporary access
	ModeSigned AccessMode = "signed"
	// ModeProxy streams objects through the gateway
	ModeProxy AccessMode = "proxy"
)

// PresignRequest represents a request to generate a presigned URL
type PresignRequest struct {
	Bucket     string `json:"bucket" binding:"required"`
	Key        string `json:"key" binding:"required"`
	Expiration int    `json:"expiration"` // Expiration time in seconds, default 3600
	Method     string `json:"method"`     // HTTP method: GET, PUT, default GET
}

// PresignResponse represents the response containing a presigned URL
type PresignResponse struct {
	URL        string `json:"url"`
	Expiration int    `json:"expiration"` // Expiration time in seconds
}

// DirectURLResponse represents the response for direct mode
type DirectURLResponse struct {
	URL string `json:"url"`
}

// BucketConfig represents per-bucket access mode configuration
type BucketConfig struct {
	AccessMode AccessMode `json:"access_mode"`
}

type AddressingStyle string

const (
	AddressingVirtualHost AddressingStyle = "virtual-host"
	AddressingPathStyle   AddressingStyle = "path"
)

// StorageObjectResponse represents the response for storage gateway object operations
type StorageObjectResponse struct {
	Mode               AccessMode
	URL                string
	RedirectURL        string
	Stream             io.ReadCloser
	ContentType        string
	Size               int64
	ETag               string
	LastModified       time.Time
	ContentRange       string
	CacheControl       string
	ContentDisposition string
}

// StorageObjectInfo represents storage gateway object metadata
type StorageObjectInfo struct {
	ContentType   string
	ContentLength int64
	LastModified  time.Time
	ETag          string
}

// StoragePutObjectOptions represents options for putting an object in storage gateway
type StoragePutObjectOptions struct {
	Size               int64
	ContentType        string
	ContentDisposition string
	CacheControl       string
}
