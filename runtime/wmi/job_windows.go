//go:build windows

package wmi

import (
	"context"
	"fmt"
	"time"
)

// DefaultJobPollInterval is how often WaitJob polls a started job.
const DefaultJobPollInterval = 200 * time.Millisecond

// WaitJob resolves the (ReturnValue, Job) pair every CIM async method
// returns: ReturnCompleted (0) is done, ReturnJobStarted (4096) polls the
// CIM_ConcreteJob at jobPath to a terminal state, and anything else — or a
// job that fails — is a *JobError. what is the qualified method name used in
// errors, e.g. "Msvm_ComputerSystem.RequestStateChange". Cancelling ctx
// abandons the wait, not the job.
func (s *Service) WaitJob(ctx context.Context, what string, returnValue uint32, jobPath string) error {
	return s.WaitJobEvery(ctx, what, returnValue, jobPath, DefaultJobPollInterval)
}

// WaitJobEvery is WaitJob with a custom poll interval.
func (s *Service) WaitJobEvery(ctx context.Context, what string, returnValue uint32, jobPath string, interval time.Duration) error {
	switch returnValue {
	case ReturnCompleted:
		return nil
	case ReturnJobStarted:
		// Fall through to polling.
	default:
		return &JobError{What: what, ReturnValue: returnValue}
	}
	if jobPath == "" {
		return fmt.Errorf("wmi: %s: job started but no job reference returned", what)
	}
	for {
		job, err := s.GetInstance(jobPath)
		if err != nil {
			return fmt.Errorf("wmi: %s: job poll: %w", what, err)
		}
		switch state := AsInt64(job["JobState"]); state {
		case JobStateCompleted, JobStateCompletedWithWarnings:
			return nil
		case JobStateTerminated, JobStateKilled, JobStateException:
			return &JobError{
				What:        what,
				ReturnValue: returnValue,
				JobPath:     jobPath,
				JobState:    state,
				ErrorCode:   AsInt64(job["ErrorCode"]),
				Description: AsString(job["ErrorDescription"]),
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wmi: %s: %w", what, ctx.Err())
		case <-time.After(interval):
		}
	}
}
