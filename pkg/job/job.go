package job

import (
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/MinhNghiaD/jobworker/api/worker/proto"
	"github.com/MinhNghiaD/jobworker/pkg/log"
)

// Job is managed by the worker for the execution of linux process
type Job interface {
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

// Impl is the implementation of the Job interface
type Impl struct {
	id         uuid.UUID
	cmd        *exec.Cmd
	logger     *log.Logger
	owner      string
	state      proto.ProcessState
	stateMutex sync.RWMutex
	exitChan   chan bool
}

// newJob creates a new job, associated with a unique ID and a Command, and the owner who requests its
func newJob(ID uuid.UUID, cmd *exec.Cmd, logger *log.Logger, owner string) (Job, error) {
	return &Impl{
		id:       ID,
		cmd:      cmd,
		logger:   logger,
		owner:    owner,
		state:    proto.ProcessState_RUNNING,
		exitChan: make(chan bool, 1),
	}, nil
}

// Start runs job in the background
func (j *Impl) Start() error {
	// IO Mapping
	stdoutLog := j.logger.Entry.WithField("source", "stdout").Writer()
	stderrLog := j.logger.Entry.WithField("source", "stderr").Writer()

	j.cmd.Stdout = stdoutLog
	j.cmd.Stderr = stderrLog
	j.cmd.Stdin = nil

	logrus.Infof("Starting job %s", j.id)

	err := j.cmd.Start()
	if err != nil {
		j.changeState(proto.ProcessState_STOPPED)

		badRequest := &errdetails.BadRequest{
			FieldViolations: []*errdetails.BadRequest_FieldViolation{
				{
					Field:       "command",
					Description: err.Error(),
				},
			},
		}

		return ReportError(codes.PermissionDenied, "Fail to start job", badRequest)
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

// Stop terminates a job. If the force flag is set to true, it will using SIGKILL to terminate the process, otherwise it will be SIGTERM
func (j *Impl) Stop(force bool) error {
	if j.getState() != proto.ProcessState_RUNNING || j.cmd.Process == nil {
		badRequest := &errdetails.BadRequest{
			FieldViolations: []*errdetails.BadRequest_FieldViolation{
				{
					Field:       "job",
					Description: "Job not running",
				},
			},
		}

		return ReportError(codes.AlreadyExists, "Fail to stop job", badRequest)
	}

	// Use SIGKILL to force the process to stop immediately, otherwise we will use SIGTERM
	if force {
		if err := j.cmd.Process.Signal(os.Kill); err != nil {
			badRequest := &errdetails.BadRequest{
				FieldViolations: []*errdetails.BadRequest_FieldViolation{
					{
						Field:       "job",
						Description: err.Error(),
					},
				},
			}

			return ReportError(codes.Internal, "Fail to stop job", badRequest)
		}
	} else {
		if err := j.cmd.Process.Signal(syscall.SIGTERM); err != nil {
			badRequest := &errdetails.BadRequest{
				FieldViolations: []*errdetails.BadRequest_FieldViolation{
					{
						Field:       "job",
						Description: err.Error(),
					},
				},
			}

			return ReportError(codes.Internal, "Fail to stop job", badRequest)
		}
	}

	// Wait for the job to exit with timeout of 1s
	select {
	case _, ok := <-j.exitChan:
		if !ok {
			// exit channel is already closed, which means the process is already exited
			badRequest := &errdetails.BadRequest{
				FieldViolations: []*errdetails.BadRequest_FieldViolation{
					{
						Field:       "job",
						Description: "Job not running",
					},
				},
			}

			return ReportError(codes.AlreadyExists, "Fail to stop job", badRequest)
		}
	case <-time.After(time.Second):
		badRequest := &errdetails.BadRequest{
			FieldViolations: []*errdetails.BadRequest_FieldViolation{
				{
					Field:       "force",
					Description: "Job takes too long to stop",
				},
			},
		}

		return ReportError(codes.DeadlineExceeded, "Fail to stop job", badRequest)
	}

	j.changeState(proto.ProcessState_STOPPED)

	return nil
}

// Status queries the current statis of the job
func (j *Impl) Status() *proto.JobStatus {
	state := j.getState()
	pid := -1
	exitcode := -1

	if j.cmd.Process != nil {
		pid = j.cmd.Process.Pid
	}

	// if state > proto.ProcessState_RUNNING, the process is already terminated and ProcessState can be read safely
	if state > proto.ProcessState_RUNNING && j.cmd.ProcessState != nil {
		exitcode = j.cmd.ProcessState.ExitCode()
	}

	return &proto.JobStatus{
		Job:     &proto.Job{Id: j.id.String()},
		Command: &proto.Command{Cmd: j.String()},
		Owner:   j.owner,
		Status: &proto.ProcessStatus{
			Pid:      int32(pid),
			State:    state,
			ExitCode: int32(exitcode),
		},
	}
}

// String returns the command wrapped by the job
func (j *Impl) String() string {
	return j.cmd.String()
}

// getState reads current job state with mutex read protection
func (j *Impl) getState() proto.ProcessState {
	j.stateMutex.RLock()
	defer j.stateMutex.RUnlock()

	return j.state
}

// changeState sets the new state of the job
func (j *Impl) changeState(state proto.ProcessState) {
	j.stateMutex.Lock()
	defer j.stateMutex.Unlock()

	j.state = state
}

func ReportError(code codes.Code, desc string, details *errdetails.BadRequest) error {
	st := status.New(code, desc)
	st, err := st.WithDetails(details)
	if err != nil {
		logrus.Errorf("Fail to generate error, err %s", err)
		return status.Errorf(code, desc)
	}

	return st.Err()
}
