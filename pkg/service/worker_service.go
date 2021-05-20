package service

import (
	"context"
	"fmt"

	"github.com/MinhNghiaD/jobworker/api/worker/proto"
)

type WorkerService struct {
	proto.UnimplementedWorkerServiceServer
}

func NewWorkerService() (*WorkerService, error) {
	return &WorkerService{}, nil
}

func (service *WorkerService) StartJob(ctx context.Context, cmd *proto.Command) (*proto.Job, error) {
	return nil, fmt.Errorf("Unimplemented")
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
