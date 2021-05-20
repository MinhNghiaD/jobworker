package job

import (
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

// Job status, which includes PID, Job state and Process exit code
type ProcStat struct {
	PID      int
	Stat     State
	ExitCode int
}

// Job is managed by the worker for the execution of linux process
type Job interface {
	// Start the job in the background
	Start() error
	// Stop the job
	Stop(force bool) error
	// Query the current status of the job
	Status() ProcStat
}

// The implementation of Job interface
type JobImpl struct {
	ID         uuid.UUID
	cmd        *exec.Cmd
	state      State
	stateMutex sync.RWMutex
	waitExit   sync.WaitGroup
}

// Create a new job, associated with a unique ID and a Command
func newJob(ID uuid.UUID, cmd *exec.Cmd) (Job, error) {
	return &JobImpl{
		ID:    ID,
		cmd:   cmd,
		state: QUEUING,
	}, nil
}

// Start Running job in the background. The Start() function has a timeout period of 1 second
func (j *JobImpl) Start() error {
	j.cmd.Stdin = nil
	j.cmd.Stdout = os.Stdout
	j.cmd.Stderr = os.Stdout

	j.cmd.SysProcAttr = configNameSpace()

	logrus.Infof("Starting j %s", j.ID)

	err := j.cmd.Start()

	if err != nil {
		j.changeState(STOPPED)
		return err
	}

	j.changeState(RUNNING)

	j.waitExit.Add(1)

	go func() {
		defer j.waitExit.Done()

		if err := j.cmd.Wait(); err != nil {
			logrus.Infof("Job %s terminated, reason %s", j.ID, err)
		}

		logrus.Infof("Exiting job %s", j.ID)

		j.changeState(EXITED)
		/*
			if j.cmd.ProcessState != nil && (j.cmd.ProcessState.Exited() != false || j.cmd.ProcessState.ExitCode() < 0) {
				j.changeState(STOPPED)
			} else {

			}
		*/
	}()

	return nil
}

// Stop a job. If the force flag is set to true, it will using SIGKILL to terminate the process, otherwise it will be SIGTERM
func (j *JobImpl) Stop(force bool) error {
	if j.getState() != RUNNING || j.cmd.Process == nil {
		return fmt.Errorf("Job not running")
	}

	if force {
		if err := j.cmd.Process.Signal(os.Kill); err != nil {
			return err
		}
	} else {
		if err := j.cmd.Process.Signal(syscall.SIGTERM); err != nil {
			return err
		}
	}

	// Wait for the job to terminated
	timeoutChan := make(chan error, 1)
	go func() {
		defer close(timeoutChan)
		j.waitExit.Wait()
	}()

	select {
	case <-timeoutChan:
	case <-time.After(time.Second):
		logrus.Infof("Stop job %s deadline excedeed", j)
		return fmt.Errorf("Fail to stop job, timeout")
	}

	j.changeState(STOPPED)

	return nil
}

// Query the current statis of the job
func (j *JobImpl) Status() ProcStat {
	pid := -1
	exitcode := -1

	if j.cmd.Process != nil {
		pid = j.cmd.Process.Pid
	}

	if j.cmd.ProcessState != nil {
		exitcode = j.cmd.ProcessState.ExitCode()
	}

	return ProcStat{
		PID:      pid,
		Stat:     j.getState(),
		ExitCode: exitcode,
	}
}

// Return the command wrapped by the job
func (j *JobImpl) String() string {
	return j.cmd.String()
}

// Read current job state with mutex read protection
func (j *JobImpl) getState() State {
	j.stateMutex.RLock()
	defer j.stateMutex.RUnlock()

	return j.state
}

// Change the current state of the job
func (j *JobImpl) changeState(state State) {
	j.stateMutex.Lock()
	defer j.stateMutex.Unlock()

	j.state = state
}

// Apply namespaces policy on the process
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
