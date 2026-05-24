package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/shawn/jobhunttask/internal/model"
	"github.com/shawn/jobhunttask/internal/service"
)

func TestSlogNotifier_WritesStructuredLog(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	n := service.NewSlogNotifier(slog.New(slog.NewJSONHandler(&buf, nil)))
	r := &model.Reminder{
		ID:      uuid.New(),
		Kind:    model.ReminderKindMorning,
		Payload: map[string]any{"message": "go!"},
	}
	if err := n.Notify(context.Background(), r); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if !strings.Contains(buf.String(), `"msg":"reminder dispatched"`) {
		t.Errorf("missing msg: %s", buf.String())
	}
	// Round-trip parse to confirm it's valid JSON.
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if got["kind"] != "morning" {
		t.Errorf("kind=%v", got["kind"])
	}
}
