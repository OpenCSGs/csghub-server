package types

import "encoding/json"

// CSGClawGuestUsageMaxCount returns the limited guest policy maximum from
// instance metadata. Nil means the policy is absent, malformed, or unlimited.
func CSGClawGuestUsageMaxCount(metadata *map[string]any) *int64 {
	if metadata == nil {
		return nil
	}
	provisionRequest, ok := (*metadata)["provision_request"].(map[string]any)
	if !ok {
		return nil
	}
	raw, ok := provisionRequest["guest_usage_limit"]
	if !ok || raw == nil {
		return nil
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var limit GuestUsageLimit
	if err := json.Unmarshal(encoded, &limit); err != nil {
		return nil
	}
	if limit.Mode != GuestUsageLimitLimited {
		return nil
	}
	maxCount := limit.MaxCount
	return &maxCount
}
