package job

import (
	"context"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	exec "golang.org/x/sys/execabs"

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
	return &JobImpl{
		ID:    ID,
		cmd:   cmd,
		state: QUEUING,
	}, nil
}

func (j *JobImpl) Start() (err error) {
	j.cmd.Stdin = nil
	j.cmd.Stdout = os.Stdout
	j.cmd.Stderr = os.Stdout

	j.cmd.SysProcAttr = configNameSpace()

	logrus.Infof("Starting j %s", j.ID)

	ctx, cancel := context.WithTimeout(context.TODO(), time.Second)
	defer cancel()

	errChan := make(chan error, 1)
	defer close(errChan)

	go func() {
		errChan <- j.cmd.Start()
	}()

	select {
	case err = <-errChan:
	case <-ctx.Done():
		logrus.Infof("Start job %s deadline excedeed", j)
		err = ctx.Err()
	}

	if err != nil {
		j.changeState(STOPPED)
		return err
	}

	j.changeState(RUNNING)

	go func() {
		if err := j.cmd.Wait(); err != nil {
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

func (j *JobImpl) String() string {
	return j.cmd.String()
}

func (j *JobImpl) changeState(state State) {
	j.stateMutex.Lock()
	defer j.stateMutex.Unlock()

	j.state = state
}

func configNameSpace() *syscall.SysProcAttr {
	// TODO Continue to reinforce namespaces
	return &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWNS |
			syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNET |
			syscall.CLONE_NEWUSER,
		UidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getuid(),
				Size:        1,
			},
		},
		GidMappings: []syscall.SysProcIDMap{
			{
				ContainerID: 0,
				HostID:      os.Getgid(),
				Size:        1,
			},
		},
	}
}
