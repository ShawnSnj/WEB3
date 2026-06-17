package resume_test

import (
	"testing"

	"github.com/shawn/jobhunttask/internal/crm/engine/resume"
)

func TestExtractTextPlain(t *testing.T) {
	text, err := resume.ExtractText("resume.txt", "text/plain", []byte("Hello Go Kafka"))
	if err != nil {
		t.Fatal(err)
	}
	if text != "Hello Go Kafka" {
		t.Fatalf("got %q", text)
	}
}

func TestExtractTextUnsupported(t *testing.T) {
	_, err := resume.ExtractText("resume.xyz", "application/octet-stream", []byte{1, 2, 3})
	if err == nil {
		t.Fatal("expected error for unknown extension")
	}
}
