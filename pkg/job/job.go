package job

import (
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type State int

const (
	// TODO Handle QUEUING state when job queue and scheduler is implemented
	// For now, the job will be start immediately after created
	// Process waiting to be executed
	QUEUING State = iota
	// Process is running
	RUNNING
	// Process exited normally
	EXITED
	// Process stopped by a signal
	STOPPED
)

type ProcStat struct {
	PID      int
	Stat     State
	ExitCode int
}

type Job interface {
	Start() error
	Stop(force bool) error
	Status() ProcStat
}

type JobImpl struct {
	ID         uuid.UUID
	cmd        *exec.Cmd
	state      State
	stateMutex sync.RWMutex
}

func newJob(ID uuid.UUID, cmd *exec.Cmd) (Job, error) {
	// TODO Using namespaces to limit the effects of process on the system
	return &JobImpl{
		ID:    ID,
		cmd:   cmd,
		state: QUEUING,
	}, nil
}

func (j *JobImpl) Start() error {
	j.cmd.Stdout = os.Stdout
	j.cmd.Stderr = os.Stdout

	logrus.Infof("Starting j %s", j.ID)

	err := j.cmd.Start()

	if err != nil {
		j.changeState(STOPPED)
		return err
	}

	j.changeState(RUNNING)

	go func() {
		if err := j.cmd.Wait(); err != nil {
			// TODO error handling
			logrus.Infof("Job %s terminated, reason %s", j.ID, err)
		}

		logrus.Infof("Exiting job %s", j.ID)

		if j.cmd.ProcessState != nil && (j.cmd.ProcessState.Exited() != false || j.cmd.ProcessState.ExitCode() < 0) {
			j.changeState(STOPPED)
		} else {
			j.changeState(EXITED)
		}
	}()

	return nil
}

func (j *JobImpl) Stop(force bool) error {
	return fmt.Errorf("Not implemented")
}

func (j *JobImpl) Status() ProcStat {
	return ProcStat{}
}

func (j *JobImpl) changeState(state State) {
	j.stateMutex.Lock()
	defer j.stateMutex.Unlock()

	j.state = state
}
