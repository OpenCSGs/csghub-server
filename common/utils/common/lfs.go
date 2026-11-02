package common

// LfsPartSize returns the effective multipart size in bytes from a MiB config.
// Uploaders and progress readers must use this same derivation for cache keys.
// It caps multipart uploads at 1000 parts and returns zero for invalid config.
func LfsPartSize(configuredPartSize int, objectSize int64) int64 {
	if configuredPartSize <= 0 {
		return 0
	}
	const maxPartNum int64 = 1000
	partSize := int64(configuredPartSize) * 1024 * 1024
	if objectSize > 0 && (objectSize-1)/partSize >= maxPartNum {
		partSize = (objectSize-1)/maxPartNum + 1
		const minPartSize = 5 * 1024 * 1024
		if partSize < minPartSize {
			partSize = minPartSize
		}
	}
	return partSize
}
