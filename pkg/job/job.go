package job

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/MinhNghiaD/jobworker/api/worker/proto"
	"github.com/MinhNghiaD/jobworker/pkg/log"
)

type State int

// Job is managed by the worker for the execution of linux process
type Job interface {
	// ID of job
	ID() string
	// Start the job in the background
	Start() error
	// Stop the job
	Stop(force bool) error
	// Query the current status of the job
	Status() *proto.JobStatus
}

// TODO Handle QUEUING state when job queue and scheduler is implemented
// For now, the job will be start immediately after created
// Process waiting to be executed

// The implementation of Job interface
type JobImpl struct {
	id         uuid.UUID
	cmd        *exec.Cmd
	logger     *log.Logger
	owner      string
	state      proto.ProcessState
	stateMutex sync.RWMutex
	exitChan   chan bool
}

// Create a new job, associated with a unique ID and a Command
func newJob(ID uuid.UUID, cmd *exec.Cmd, logger *log.Logger, owner string) (Job, error) {
	return &JobImpl{
		id:       ID,
		cmd:      cmd,
		logger:   logger,
		owner:    owner,
		state:    proto.ProcessState_RUNNING,
		exitChan: make(chan bool, 1),
	}, nil
}

func (j *JobImpl) ID() string {
	return j.id.String()
}

// Start Running job in the background. The Start() function has a timeout period of 1 second
func (j *JobImpl) Start() error {
	// IO Mapping
	j.cmd.Stdin = nil
	stdoutLog := j.logger.Entry.WithField("source", "stdout").Writer()
	stderrLog := j.logger.Entry.WithField("source", "stderr").Writer()

	j.cmd.Stdout = stdoutLog
	j.cmd.Stderr = stderrLog

	j.cmd.SysProcAttr = configNameSpace()

	logrus.Infof("Starting job %s", j.id)

	err := j.cmd.Start()

	if err != nil {
		j.changeState(proto.ProcessState_STOPPED)
		return err
	}

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

		j.changeState(proto.ProcessState_EXITED)

		// Inform stop caller to finish.
		// The channel buffer size is 1, so if this job exit naturally, it won't be blocked
		j.exitChan <- true
		close(j.exitChan)
	}()

	return nil
}

// Stop a job. If the force flag is set to true, it will using SIGKILL to terminate the process, otherwise it will be SIGTERM
func (j *JobImpl) Stop(force bool) error {
	if j.getState() != proto.ProcessState_RUNNING || j.cmd.Process == nil {
		return fmt.Errorf("Job not running")
	}

	// Use SIGKILL to force the process to stop immediately, otherwise we will use SIGTERM
	if force {
		if err := j.cmd.Process.Signal(os.Kill); err != nil {
			return err
		}
	} else {
		if err := j.cmd.Process.Signal(syscall.SIGTERM); err != nil {
			return err
		}
	}

	// Wait for the job to exit with timeout of 1s
	select {
	case _, ok := <-j.exitChan:
		if !ok {
			// exit channel is already closed, which means the process is already exited
			return fmt.Errorf("Job is already stopped")
		}
	case <-time.After(time.Second):
		return fmt.Errorf("Fail to stop job, timeout")
	}

	j.changeState(proto.ProcessState_STOPPED)

	return nil
}

// Query the current statis of the job
func (j *JobImpl) Status() *proto.JobStatus {
	state := j.getState()
	pid := -1
	exitcode := -1

	if j.cmd.Process != nil {
		pid = j.cmd.Process.Pid
	}

	if state > proto.ProcessState_RUNNING && j.cmd.ProcessState != nil {
		exitcode = j.cmd.ProcessState.ExitCode()
	}

	return &proto.JobStatus{
		Job:     &proto.Job{Id: j.ID()},
		Command: &proto.Command{Cmd: j.String()},
		Owner:   j.owner,
		Status: &proto.ProcessStatus{
			Pid:      int32(pid),
			State:    state,
			ExitCode: int32(exitcode),
		},
	}
}

// Return the command wrapped by the job
func (j *JobImpl) String() string {
	return j.cmd.String()
}

// Read current job state with mutex read protection
func (j *JobImpl) getState() proto.ProcessState {
	j.stateMutex.RLock()
	defer j.stateMutex.RUnlock()

	return j.state
}

// Change the current state of the job
func (j *JobImpl) changeState(state proto.ProcessState) {
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
