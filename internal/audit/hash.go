package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"
)

func BuildEvent(drillID string, sequence int, eventType string, payload any, occurredAt time.Time, previous string) (Event, error) {
	canonical, err := CanonicalJSON(payload)
	if err != nil {
		return Event{}, err
	}
	e := Event{DrillID: drillID, Sequence: sequence, EventType: eventType, Payload: canonical, OccurredAt: occurredAt.UTC(), PreviousHash: previous}
	e.CurrentHash = eventHash(e)
	return e, nil
}

func eventHash(e Event) string {
	h := sha256.New()
	h.Write([]byte(e.PreviousHash))
	h.Write([]byte{0})
	h.Write([]byte(e.DrillID))
	h.Write([]byte{0})
	h.Write([]byte(strconv.Itoa(e.Sequence)))
	h.Write([]byte{0})
	h.Write([]byte(e.EventType))
	h.Write([]byte{0})
	h.Write(e.Payload)
	h.Write([]byte{0})
	h.Write([]byte(e.OccurredAt.UTC().Format(time.RFC3339Nano)))
	return hex.EncodeToString(h.Sum(nil))
}

func DigestDocument(value any) (string, error) {
	raw, err := CanonicalJSON(value)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:]), nil
}
