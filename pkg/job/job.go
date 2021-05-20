package job

import "fmt"

type Status int

const (
	// TODO Handle QUEUING state when job queue and scheduler is implemented
	// For now, the job will be start immediately after created
	// Process waiting to be executed
	QUEUING Status = iota
	// Process is running
	RUNNING
	// Process exited normally
	EXITED
	// Process stopped by a signal
	STOPPED
)

type ProcStat struct {
	PID      int
	State    Status
	ExitCode int
}

type Job interface {
	Start() error
	Stop(force bool) error
	Status() ProcStat
}

type JobImpl struct {
}

func (j *JobImpl) Start() error {
	return fmt.Errorf("Not implemented")
}

func (j *JobImpl) Stop(force bool) error {
	return fmt.Errorf("Not implemented")
}

func (j *JobImpl) Status() ProcStat {
	return ProcStat{}
}
