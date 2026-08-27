package audit

import "fmt"

func Verify(events []Event) error {
	previous := ""
	for index, event := range events {
		expectedSequence := index + 1
		if event.Sequence != expectedSequence {
			return &ChainError{Sequence: event.Sequence, Reason: fmt.Sprintf("审计事件序号在第 %d 项不连续", expectedSequence)}
		}
		if event.PreviousHash != previous {
			return &ChainError{Sequence: event.Sequence, Reason: fmt.Sprintf("审计事件 %d 的前序摘要不匹配", event.Sequence)}
		}
		if eventHash(event) != event.CurrentHash {
			return &ChainError{Sequence: event.Sequence, Reason: fmt.Sprintf("审计事件 %d 的摘要无效", event.Sequence)}
		}
		previous = event.CurrentHash
	}
	return nil
}
