package domain

func RecalculateProgress(a *Aggregate) {
	if a.Drill.Status.Terminal() || a.Drill.Status == StatusUnderReview || a.Drill.Status == StatusReturned {
		return
	}
	if !AllInitialResults(a) {
		a.Drill.Status = StatusExecuting
		return
	}
	if !AllDeviationsClosed(a) {
		a.Drill.Status = StatusRemediation
		return
	}
	a.Drill.Status = StatusReadyReview
}
