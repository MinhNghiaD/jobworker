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

	"github.com/MinhNghiaD/jobworker/pkg/log"
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
	// ID of job
	ID() string
	// Start the job in the background
	Start() error
	// Stop the job
	Stop(force bool) error
	// Query the current status of the job
	Status() ProcStat
}

// The implementation of Job interface
type JobImpl struct {
	id         uuid.UUID
	cmd        *exec.Cmd
	logger     *log.Logger
	state      State
	stateMutex sync.RWMutex
	exitChan   chan bool
}

// Create a new job, associated with a unique ID and a Command
func newJob(ID uuid.UUID, cmd *exec.Cmd, logger *log.Logger) (Job, error) {
	return &JobImpl{
		id:       ID,
		cmd:      cmd,
		logger:   logger,
		state:    QUEUING,
		exitChan: make(chan bool, 1),
	}, nil
}

func (j *JobImpl) ID() string {
	return j.id.String()
}

// Start Running job in the background. The Start() function has a timeout period of 1 second
func (j *JobImpl) Start() error {
	j.cmd.Stdin = nil
	stdoutLog := j.logger.Entry.WithField("source", "stdout").Writer()
	stderrLog := j.logger.Entry.WithField("source", "stderr").Writer()

	j.cmd.Stdout = stdoutLog
	j.cmd.Stderr = stderrLog

	j.cmd.SysProcAttr = configNameSpace()

	logrus.Infof("Starting job %s", j.id)

	err := j.cmd.Start()

	if err != nil {
		j.changeState(STOPPED)
		return err
	}

	j.changeState(RUNNING)

	go func() {
		if err := j.cmd.Wait(); err != nil {
			j.logger.Entry.Infof("Job %s terminated, reason %s", j.id, err)
		}

		// Close logger
		if err := stdoutLog.Close(); err != nil {
			logrus.Errorln(err)
		}

		if err := stderrLog.Close(); err != nil {
			logrus.Errorln(err)
		}

		j.logger.Entry.Infof("Exiting job %s", j.id)

		if err := j.logger.Close(); err != nil {
			logrus.Errorln(err)
		}

		j.changeState(EXITED)

		// Inform stop caller to finish.
		// The channel buffer size is 1, so if this job exit naturally, it won't be blocked
		j.exitChan <- true
		close(j.exitChan)
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

	select {
	case _, ok := <-j.exitChan:
		if !ok {
			return fmt.Errorf("Job is already stopped")
		}
	case <-time.After(time.Second):
		return fmt.Errorf("Fail to stop job, timeout")
	}

	j.changeState(STOPPED)

	return nil
}

// Query the current statis of the job
func (j *JobImpl) Status() ProcStat {
	state := j.getState()

	pid := -1
	exitcode := -1

	if state > QUEUING && j.cmd.Process != nil {
		pid = j.cmd.Process.Pid
	}

	if state > RUNNING && j.cmd.ProcessState != nil {
		exitcode = j.cmd.ProcessState.ExitCode()
	}

	return ProcStat{
		PID:      pid,
		Stat:     state,
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
