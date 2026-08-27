package audit

import (
	"testing"
	"time"
)

func TestVerifyDetectsBrokenChain(t *testing.T) {
	now := time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)
	first, err := BuildEvent("d", 1, "created", map[string]any{"value": 1}, now, "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildEvent("d", 2, "frozen", map[string]any{"value": 2}, now.Add(time.Minute), first.CurrentHash)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify([]Event{first, second}); err != nil {
		t.Fatalf("有效链被拒绝: %v", err)
	}
	second.Payload = []byte(`{"value":3}`)
	if err := Verify([]Event{first, second}); err == nil {
		t.Fatal("篡改后的审计事件未被发现")
	}
}

func TestCanonicalDocumentDigestStable(t *testing.T) {
	value := struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}{"场所", 3}
	one, err := DigestDocument(value)
	if err != nil {
		t.Fatal(err)
	}
	two, err := DigestDocument(value)
	if err != nil {
		t.Fatal(err)
	}
	if one != two || len(one) != 64 {
		t.Fatalf("摘要不稳定: %s %s", one, two)
	}
}
