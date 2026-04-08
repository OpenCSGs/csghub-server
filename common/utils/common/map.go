package common

// MergeMapWithDeletion merges src into dst.
// If a value in src is nil, the key is deleted from dst.
// If the map pointed to by dst is nil, it will be initialized.
// dst must be a non-nil pointer.
func MergeMapWithDeletion(dst *map[string]any, src map[string]any) {
	if src == nil {
		return
	}
	if dst == nil {
		return
	}
	if *dst == nil {
		*dst = make(map[string]any)
	}
	for k, v := range src {
		if v == nil {
			delete(*dst, k)
		} else {
			(*dst)[k] = v
		}
	}
}
