package wmi

import (
	"strings"
	"testing"
)

func TestJobErrorMessages(t *testing.T) {
	returnErr := &JobError{What: "Msvm_ComputerSystem.RequestStateChange", ReturnValue: 32775}
	if got := returnErr.Error(); got != "wmi: Msvm_ComputerSystem.RequestStateChange: ReturnValue 32775" {
		t.Errorf("return-value form: %s", got)
	}

	jobErr := &JobError{
		What: "Msvm_VirtualSystemManagementService.DefineSystem", ReturnValue: ReturnJobStarted,
		JobPath: `Msvm_ConcreteJob.InstanceID="X"`, JobState: JobStateException,
		ErrorCode: 32768, Description: "boom",
	}
	got := jobErr.Error()
	for _, want := range []string{"job failed (state 10)", "boom", "(code 32768)"} {
		if !strings.Contains(got, want) {
			t.Errorf("job form %q missing %q", got, want)
		}
	}
}
