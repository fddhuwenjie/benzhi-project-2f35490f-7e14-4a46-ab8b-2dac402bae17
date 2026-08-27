package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"shelter-drill-gate/internal/domain"
	"shelter-drill-gate/internal/storage"
)

type Service struct {
	store *storage.Store
	locks *coordinator
	now   func() time.Time
}

type CommandMeta struct {
	RequestID       string `json:"request_id"`
	ExpectedVersion int    `json:"expected_version"`
}

type CommandResult struct {
	Status   int
	Body     []byte
	Replayed bool
}

func NewService(store *storage.Store) *Service {
	return &Service{store: store, locks: newCoordinator(), now: time.Now}
}

func (s *Service) validateMeta(meta CommandMeta) error {
	if len(strings.TrimSpace(meta.RequestID)) < 8 || len(meta.RequestID) > 128 {
		problems := &domain.ValidationErrors{}
		problems.Add("request_id", "request_id 长度应为 8 到 128 个字符")
		return problems
	}
	if meta.ExpectedVersion < 0 {
		return fmt.Errorf("expected_version: %w", domain.ErrValidation)
	}
	return nil
}

type commandFunc func(*storage.Tx, time.Time) (string, int, any, error)

func (s *Service) execute(ctx context.Context, lockKey, operation string, meta CommandMeta, fn commandFunc) (CommandResult, error) {
	if err := s.validateMeta(meta); err != nil {
		return CommandResult{}, err
	}
	if prior, err := s.store.FindResponse(ctx, meta.RequestID); err != nil {
		return CommandResult{}, err
	} else if prior != nil {
		if prior.Operation != operation {
			return CommandResult{}, domain.ErrIdempotencyKey
		}
		return CommandResult{Status: prior.Status, Body: prior.Body, Replayed: true}, nil
	}
	unlock := s.locks.lock(lockKey)
	defer unlock()
	var result CommandResult
	err := s.store.WithinTx(ctx, func(tx *storage.Tx) error {
		prior, err := tx.FindResponse(ctx, meta.RequestID)
		if err != nil {
			return err
		}
		if prior != nil {
			if prior.Operation != operation {
				return domain.ErrIdempotencyKey
			}
			result = CommandResult{Status: prior.Status, Body: prior.Body, Replayed: true}
			return nil
		}
		now := s.now().UTC()
		drillID, status, payload, err := fn(tx, now)
		if err != nil {
			return err
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		if status == 0 {
			status = http.StatusOK
		}
		if err := tx.SaveResponse(ctx, storage.StoredResponse{RequestID: meta.RequestID, DrillID: drillID, Operation: operation, Status: status, Body: body, CreatedAt: now}); err != nil {
			return err
		}
		result = CommandResult{Status: status, Body: body}
		return nil
	})
	return result, err
}

func checkVersion(actual int, meta CommandMeta) error {
	if actual != meta.ExpectedVersion {
		return domain.ErrConflict
	}
	return nil
}

func IsClientError(err error) bool {
	return errors.Is(err, domain.ErrValidation) || errors.Is(err, domain.ErrInvalidState) ||
		errors.Is(err, domain.ErrConflict) || errors.Is(err, domain.ErrIdempotencyKey) || errors.Is(err, domain.ErrNotFound) ||
		errors.Is(err, domain.ErrDecisionUnavailable)
}
