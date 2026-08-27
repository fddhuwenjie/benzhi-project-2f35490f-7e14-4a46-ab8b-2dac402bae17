package audit

import "time"

type Event struct {
	DrillID      string    `json:"drill_id"`
	Sequence     int       `json:"sequence"`
	EventType    string    `json:"event_type"`
	Payload      []byte    `json:"payload"`
	OccurredAt   time.Time `json:"occurred_at"`
	PreviousHash string    `json:"previous_hash"`
	CurrentHash  string    `json:"current_hash"`
}

type ChainError struct {
	Sequence int
	Reason   string
}

func (e *ChainError) Error() string { return e.Reason }
