package service

import (
	"context"
	"fmt"

	"github.com/MinhNghiaD/jobworker/api/worker/proto"
	"github.com/MinhNghiaD/jobworker/pkg/job"
)

type WorkerService struct {
	proto.UnimplementedWorkerServiceServer
	jobsManager job.JobsManager
}

func NewWorkerService() (*WorkerService, error) {
	jobsManager, err := job.NewManager()

	if err != nil {
		return nil, err
	}

	return &WorkerService{
		jobsManager: jobsManager,
	}, nil
}

func (service *WorkerService) StartJob(ctx context.Context, cmd *proto.Command) (*proto.Job, error) {
	if service.jobsManager == nil {
		return nil, fmt.Errorf("Job Managers is not ready")
	}

	// TODO Get Owner common name from certificate
	jobID, err := service.jobsManager.CreateJob(cmd.Cmd, cmd.Args, "User CN")

	if err != nil {
		return nil, err
	}

	return &proto.Job{
		Id: jobID,
	}, nil
}

func (service *WorkerService) StopJob(ctx context.Context, request *proto.StopRequest) (*proto.JobStatus, error) {
	if service.jobsManager == nil {
		return nil, fmt.Errorf("Job Managers is not ready")
	}

	j, ok := service.jobsManager.GetJob(request.Job.Id)

	if !ok {
		return nil, fmt.Errorf("Job not found")
	}

	if err := j.Stop(request.Force); err != nil {
		return nil, err
	}

	status := j.Status()

	return &proto.JobStatus{
		Job:     request.Job,
		Command: &proto.Command{Cmd: status.Cmd},
		Owner:   status.Owner,
		Status: &proto.ProcessStatus{
			Pid:      int32(status.PID),
			State:    protoState(status.Stat),
			ExitCode: int32(status.ExitCode),
		},
	}, nil
}

func (service *WorkerService) QueryJob(ctx context.Context, protoJob *proto.Job) (*proto.JobStatus, error) {
	if service.jobsManager == nil {
		return nil, fmt.Errorf("Job Managers is not ready")
	}

	// TODO Get Owner common name from certificate
	j, ok := service.jobsManager.GetJob(protoJob.Id)

	if !ok {
		return nil, fmt.Errorf("Job not found")
	}

	status := j.Status()

	return &proto.JobStatus{
		Job:     protoJob,
		Command: &proto.Command{Cmd: status.Cmd},
		Owner:   status.Owner,
		Status: &proto.ProcessStatus{
			Pid:      int32(status.PID),
			State:    protoState(status.Stat),
			ExitCode: int32(status.ExitCode),
		},
	}, nil
}

func (service *WorkerService) StreamLog(job *proto.Job, stream proto.WorkerService_StreamLogServer) error {
	return fmt.Errorf("Unimplemented")
}

func (service *WorkerService) Cleanup() error {
	if service.jobsManager == nil {
		return fmt.Errorf("Job Managers is not ready")
	}

	return service.jobsManager.Cleanup()
}

func protoState(state job.State) proto.ProcessState {
	switch state {
	case job.EXITED:
		return proto.ProcessState_EXITED
	case job.STOPPED:
		return proto.ProcessState_STOPPED
	}

	return proto.ProcessState_RUNNING
}
