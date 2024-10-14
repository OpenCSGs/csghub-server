package s3

import (
	"context"
	"testing"
)

// test for useInternalClient
func Test_useInternalClient(t *testing.T) {
	c := &Client{}
	ctx := context.Background()
	if c.useInternalClient(ctx) != false {
		t.Errorf("should not use internal s3 client if context not defined")
	}
	ctxWrongValue := context.WithValue(ctx, "X-OPENCSG-S3-Internal", "test")
	if c.useInternalClient(ctxWrongValue) != false {
		t.Errorf("should not use internal s3 client if context value is not 'true'")
	}
	ctxWithRightValue := context.WithValue(ctx, "X-OPENCSG-S3-Internal", true)
	if c.useInternalClient(ctxWithRightValue) != true {
		t.Errorf("should not use internal s3 client if context value is 'true'")
	}

}
