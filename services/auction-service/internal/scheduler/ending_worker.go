package scheduler

// EndingWorker logic is integrated into SchedulerService.EndAuction via EndingService.
// This file documents the worker pattern for horizontal scaling with Redis locks.

// RunEndingWorker is an alias hook for dedicated ending pods (optional K8s deployment).
func RunEndingWorker() {
	// Use AuctionScheduler.Run — single ticker handles activate + end.
}
