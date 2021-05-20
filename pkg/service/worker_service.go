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

	jobID, err := service.jobsManager.CreateJob(cmd.Cmd, cmd.Args)

	if err != nil {
		return nil, err
	}

	return &proto.Job{
		Id: jobID,
	}, nil
}

func (service *WorkerService) StopJob(ctx context.Context, request *proto.StopRequest) (*proto.JobStatus, error) {
	return nil, fmt.Errorf("Unimplemented")
}

func (service *WorkerService) QueryJob(ctx context.Context, job *proto.Job) (*proto.JobStatus, error) {
	return nil, fmt.Errorf("Unimplemented")
}

func (service *WorkerService) StreamLog(job *proto.Job, stream proto.WorkerService_StreamLogServer) error {
	return fmt.Errorf("Unimplemented")
}
