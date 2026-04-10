package client

import (
	"bytes"
	"encoding/base64"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/gomail.v2"
)

// Ensures SMTP attachments can be built from in-memory bytes (same pattern as emailService.Send).
func TestGomailAttachEmptyPathWithSetCopyFunc(t *testing.T) {
	msg := gomail.NewMessage()
	msg.SetHeader("From", "a@b.c")
	msg.SetHeader("To", "d@e.f")
	msg.SetHeader("Subject", "s")
	msg.SetBody("text/plain", "body")

	payload := []byte("col1,col2\nv1,v2\n")
	msg.Attach("", gomail.Rename("report.csv"), gomail.SetCopyFunc(func(w io.Writer) error {
		_, err := w.Write(payload)
		return err
	}))

	var buf bytes.Buffer
	_, err := msg.WriteTo(&buf)
	assert.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "report.csv")
	// Attachment body is base64 in the MIME output
	encoded := base64.StdEncoding.EncodeToString(payload)
	assert.Contains(t, out, encoded)
}
