package config

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7/pkg/s3utils"
)

const (
	s3DualstackPrefix = "s3.dualstack."
	// s3GlobalEndpoint is the region-less AWS global endpoint. minio-go never
	// signs against it — it signs the regional host derived from the configured
	// region — so it is normalized with the configured region as fallback.
	s3GlobalEndpoint = "s3.amazonaws.com"
)

// NormalizeS3Endpoint rewrites an Amazon S3 endpoint host to its dual-stack form
// (or back) so the host minio-go signs agrees with the host the storage gateway
// reconstructs from the configured endpoint. Non-Amazon endpoints, FIPS,
// PrivateLink, and S3 Express endpoints (which minio-go never rewrites or routes
// through the regular S3 host selection) are returned unchanged, as are hosts
// whose region can be derived neither from the endpoint nor from fallbackRegion.
func NormalizeS3Endpoint(endpoint string, enableDualstack bool, fallbackRegion string) string {
	if endpoint == "" {
		return endpoint
	}
	raw := endpoint
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return endpoint
	}
	if !s3utils.IsAmazonEndpoint(*u) ||
		s3utils.IsAmazonFIPSEndpoint(*u) ||
		s3utils.IsAmazonPrivateLinkEndpoint(*u) ||
		s3utils.IsAmazonExpressRegionalEndpoint(*u) ||
		s3utils.IsAmazonExpressZonalEndpoint(*u) {
		return endpoint
	}

	host := u.Host
	if !enableDualstack {
		if strings.HasPrefix(host, s3DualstackPrefix) {
			return strings.Replace(endpoint, s3DualstackPrefix, "s3.", 1)
		}
		// The minio-go SDK always signs the canonical dot-style regional host
		// for Amazon endpoints (ignoring the configured host form), so legacy
		// hyphen-style endpoints must be canonicalized too.
		region := s3utils.GetRegionFromURL(*u)
		if region == "" {
			if host == s3GlobalEndpoint && fallbackRegion != "" {
				return strings.Replace(endpoint, host, fmt.Sprintf("s3.%s.amazonaws.com", fallbackRegion), 1)
			}
			return endpoint
		}
		i := strings.Index(host, region)
		if i < 0 {
			return endpoint
		}
		canonical := "s3." + host[i:]
		if canonical == host {
			return endpoint
		}
		return strings.Replace(endpoint, host, canonical, 1)
	}

	if strings.HasPrefix(host, s3DualstackPrefix) {
		return endpoint
	}
	if host == s3GlobalEndpoint {
		if fallbackRegion == "" {
			return endpoint
		}
		return strings.Replace(endpoint, host, s3DualstackPrefix+fallbackRegion+".amazonaws.com", 1)
	}
	region := s3utils.GetRegionFromURL(*u)
	if region == "" {
		return endpoint
	}
	i := strings.Index(host, region)
	if i < 0 {
		return endpoint
	}
	return strings.Replace(endpoint, host, s3DualstackPrefix+host[i:], 1)
}
