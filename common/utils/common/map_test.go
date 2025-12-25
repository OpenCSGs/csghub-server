package common

import (
	"testing"
)

func TestMergeMapWithDeletion(t *testing.T) {
	tests := []struct {
		name         string
		dst          *map[string]any
		src          map[string]any
		expected     *map[string]any
		expectNilDst bool
	}{
		{
			name: "merge with nil src should not modify dst",
			dst:  &map[string]any{"key1": "value1"},
			src:  nil,
			expected: &map[string]any{
				"key1": "value1",
			},
		},
		{
			name:         "merge with nil dst pointer should return early",
			dst:          nil,
			src:          map[string]any{"key1": "value1", "key2": "value2"},
			expectNilDst: true,
		},
		{
			name: "merge with nil map should initialize and merge",
			dst:  func() *map[string]any { var m map[string]any; return &m }(),
			src:  map[string]any{"key1": "value1", "key2": "value2"},
			expected: &map[string]any{
				"key1": "value1",
				"key2": "value2",
			},
		},
		{
			name: "merge should add new keys",
			dst:  &map[string]any{"key1": "value1"},
			src:  map[string]any{"key2": "value2", "key3": "value3"},
			expected: &map[string]any{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
		},
		{
			name: "merge should update existing keys",
			dst:  &map[string]any{"key1": "old_value", "key2": "value2"},
			src:  map[string]any{"key1": "new_value"},
			expected: &map[string]any{
				"key1": "new_value",
				"key2": "value2",
			},
		},
		{
			name: "merge should delete keys when value is nil",
			dst:  &map[string]any{"key1": "value1", "key2": "value2", "key3": "value3"},
			src:  map[string]any{"key2": nil},
			expected: &map[string]any{
				"key1": "value1",
				"key3": "value3",
			},
		},
		{
			name: "merge with nil map and nil values should initialize but not add nil keys",
			dst:  func() *map[string]any { var m map[string]any; return &m }(),
			src:  map[string]any{"key1": "value1", "key2": nil},
			expected: &map[string]any{
				"key1": "value1",
			},
		},
		{
			name: "merge should handle mixed operations",
			dst:  &map[string]any{"key1": "old_value", "key2": "value2", "key3": "value3"},
			src:  map[string]any{"key1": "new_value", "key2": nil, "key4": "value4"},
			expected: &map[string]any{
				"key1": "new_value",
				"key3": "value3",
				"key4": "value4",
			},
		},
		{
			name: "merge should handle empty dst",
			dst:  &map[string]any{},
			src:  map[string]any{"key1": "value1"},
			expected: &map[string]any{
				"key1": "value1",
			},
		},
		{
			name: "merge should handle empty src",
			dst:  &map[string]any{"key1": "value1"},
			src:  map[string]any{},
			expected: &map[string]any{
				"key1": "value1",
			},
		},
		{
			name: "merge should handle different value types",
			dst:  &map[string]any{"key1": "value1"},
			src:  map[string]any{"key2": 42, "key3": true, "key4": []string{"a", "b"}},
			expected: &map[string]any{
				"key1": "value1",
				"key2": 42,
				"key3": true,
				"key4": []string{"a", "b"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dst *map[string]any
			if tt.dst != nil {
				dstMap := make(map[string]any)
				for k, v := range *tt.dst {
					dstMap[k] = v
				}
				dst = &dstMap
			}
			// dst can be nil for the nil pointer test case

			MergeMapWithDeletion(dst, tt.src)

			if tt.expectNilDst {
				// Test case expects dst to remain nil
				if dst != nil {
					t.Error("expected dst to remain nil")
				}
				return
			}

			if dst == nil {
				t.Fatal("dst should not be nil after merge")
			}

			if tt.expected == nil {
				t.Fatal("test case must provide expected result or set expectNilDst")
			}

			if len(*dst) != len(*tt.expected) {
				t.Errorf("expected length %d, got %d", len(*tt.expected), len(*dst))
			}

			for k, expectedValue := range *tt.expected {
				actualValue, exists := (*dst)[k]
				if !exists {
					t.Errorf("expected key %s to exist", k)
					continue
				}
				if !equalValues(actualValue, expectedValue) {
					t.Errorf("key %s: expected %v, got %v", k, expectedValue, actualValue)
				}
			}

			for k := range *dst {
				if _, exists := (*tt.expected)[k]; !exists {
					t.Errorf("unexpected key %s in result", k)
				}
			}
		})
	}
}

// equalValues compares two values for equality, handling slices and other types
func equalValues(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// For slices, we need special handling
	if aSlice, ok := a.([]string); ok {
		if bSlice, ok := b.([]string); ok {
			if len(aSlice) != len(bSlice) {
				return false
			}
			for i := range aSlice {
				if aSlice[i] != bSlice[i] {
					return false
				}
			}
			return true
		}
		return false
	}

	return a == b
}
