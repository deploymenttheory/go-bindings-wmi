package wmi

import "fmt"

// Standard CIM method return codes for asynchronous methods (the
// (ReturnValue, Job) contract of CIM_ConcreteJob providers such as Hyper-V).
const (
	// ReturnCompleted: the method finished synchronously with no error.
	ReturnCompleted uint32 = 0
	// ReturnJobStarted: the method started a job — poll the returned
	// CIM_ConcreteJob reference to a terminal state (WaitJob does this).
	ReturnJobStarted uint32 = 4096
)

// CIM_ConcreteJob.JobState terminal values.
const (
	JobStateCompleted  int64 = 7
	JobStateTerminated int64 = 8
	JobStateKilled     int64 = 9
	JobStateException  int64 = 10
	// JobStateCompletedWithWarnings is Hyper-V's Msvm_ConcreteJob extension —
	// terminal and successful.
	JobStateCompletedWithWarnings int64 = 32768
)

// JobError is a failed CIM method call or job: a non-zero, non-job-started
// ReturnValue, or a started job that reached a failing terminal state.
type JobError struct {
	// What is the qualified method name, e.g. "Msvm_ComputerSystem.RequestStateChange".
	What string
	// ReturnValue is the method's raw return code.
	ReturnValue uint32
	// JobPath, JobState, ErrorCode, and Description are filled when a started
	// job failed (JobState is then terminal and non-successful).
	JobPath     string
	JobState    int64
	ErrorCode   int64
	Description string
}

func (e *JobError) Error() string {
	if e.JobState != 0 {
		return fmt.Sprintf("wmi: %s: job failed (state %d): %s (code %d)",
			e.What, e.JobState, e.Description, e.ErrorCode)
	}
	return fmt.Sprintf("wmi: %s: ReturnValue %d", e.What, e.ReturnValue)
}
