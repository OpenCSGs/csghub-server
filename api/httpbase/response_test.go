package httpbase

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/common/errorx"
)

// TestAccepted verifies asynchronous APIs use the standard response envelope.
func TestAccepted(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = &http.Request{}

	Accepted(ctx, map[string]int{"task_id": 42})

	require.Equal(t, http.StatusAccepted, recorder.Code)
	require.JSONEq(t, `{"msg":"Accepted","data":{"task_id":42}}`, recorder.Body.String())
}

// TestAcceptedWithExt verifies temporary asynchronous errors can include result data.
func TestAcceptedWithExt(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	AcceptedWithExt(ctx, errorx.NewCustomError("TEST", 1, errors.New("pending"), nil), map[string]int{"task_id": 42})

	require.Equal(t, http.StatusAccepted, recorder.Code)
	require.JSONEq(t, `{"code":"TEST-1","msg":"TEST-1: pending","data":{"task_id":42}}`, recorder.Body.String())
}

func TestNormalizeEmptySlice(t *testing.T) {
	tests := []struct {
		name string
		data interface{}
	}{
		{
			name: "nil interface",
			data: nil,
		},
		{
			name: "nil []string slice",
			data: []string(nil),
		},
		{
			name: "empty []string slice",
			data: []string{},
		},
		{
			name: "non-empty []string slice",
			data: []string{"a", "b"},
		},
		{
			name: "nil *[]string (nil pointer to slice)",
			data: (*[]string)(nil),
		},
		{
			name: "non-nil *[]string pointing to nil slice",
			data: func() *[]string { var s []string; return &s }(),
		},
		{
			name: "non-nil *[]string pointing to empty slice",
			data: func() *[]string { s := []string{}; return &s }(),
		},
		{
			name: "non-nil *[]string pointing to non-empty slice",
			data: func() *[]string { s := []string{"a", "b"}; return &s }(),
		},
		{
			name: "nil []int slice",
			data: []int(nil),
		},
		{
			name: "empty []int slice",
			data: []int{},
		},
		{
			name: "nil *[]int (nil pointer to slice)",
			data: (*[]int)(nil),
		},
		{
			name: "non-slice pointer (*int)",
			data: func() *int { x := 42; return &x }(),
		},
		{
			name: "nil non-slice pointer (*int)",
			data: (*int)(nil),
		},
		{
			name: "string (non-slice, non-pointer)",
			data: "hello",
		},
		{
			name: "struct (non-slice, non-pointer)",
			data: struct{ Name string }{Name: "test"},
		},
		{
			name: "nil []any slice",
			data: []any(nil),
		},
		{
			name: "nil *[]any (nil pointer to any slice)",
			data: (*[]any)(nil),
		},
		{
			name: "non-nil *[]any pointing to nil slice",
			data: func() *[]any { var s []any; return &s }(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeEmptySlice(tt.data)
			// nil interface input should stay nil
			if tt.data == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}
			// After normalization, any nil slice or pointer-to-nil-slice should become a non-nil empty slice
			originalKind := reflect.TypeOf(tt.data).Kind()
			isSliceInput := originalKind == reflect.Slice
			isPtrToSlice := originalKind == reflect.Pointer && reflect.TypeOf(tt.data).Elem().Kind() == reflect.Slice

			if isSliceInput || isPtrToSlice {
				rv := reflect.ValueOf(result)
				if rv.Kind() == reflect.Pointer {
					rv = rv.Elem()
				}
				if rv.Kind() == reflect.Slice && rv.IsNil() {
					t.Errorf("result is a nil slice, should be empty slice []")
				}
			}
		})
	}
}

func TestNormalizeEmptySlice_ReturnsEmptySliceNotNil(t *testing.T) {
	// The primary contract: nil slices and nil pointers-to-slices become non-nil empty slices

	result := normalizeEmptySlice([]string(nil))
	rv := reflect.ValueOf(result)
	if rv.Kind() != reflect.Slice || rv.IsNil() {
		t.Errorf("nil []string should become non-nil empty slice, got kind=%v isNil=%v", rv.Kind(), rv.IsNil())
	}

	result = normalizeEmptySlice((*[]string)(nil))
	rv = reflect.ValueOf(result)
	if rv.Kind() != reflect.Slice || rv.IsNil() {
		t.Errorf("nil *[]string should become non-nil empty slice, got kind=%v isNil=%v", rv.Kind(), rv.IsNil())
	}

	var nilSlice []int
	result = normalizeEmptySlice(&nilSlice)
	rv = reflect.ValueOf(result)
	if rv.Kind() != reflect.Slice || rv.IsNil() {
		t.Errorf("non-nil *[]int pointing to nil slice should become non-nil empty slice, got kind=%v isNil=%v", rv.Kind(), rv.IsNil())
	}
}
