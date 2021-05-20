package service

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/MinhNghiaD/jobworker/api/worker/proto"
	"github.com/MinhNghiaD/jobworker/pkg/job"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

type WorkerService struct {
	proto.UnimplementedWorkerServiceServer
	jobsManager job.JobsManager
}

func newWorkerService() (*WorkerService, error) {
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

	return j.Status(), nil
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

	return j.Status(), nil
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

type WorkerServer struct {
	grpcServer *grpc.Server
	listener   net.Listener
	service    *WorkerService
}

func NewServer(port int) (*WorkerServer, error) {
	logrus.Infof("Listen at port %d", port)
	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))

	if err != nil {
		return nil, err
	}

	opts := serverConfig()
	grpcServer := grpc.NewServer(opts...)

	service, err := newWorkerService()

	if err != nil {
		return nil, err
	}

	proto.RegisterWorkerServiceServer(grpcServer, service)

	return &WorkerServer{
		grpcServer: grpcServer,
		listener:   listener,
		service:    service,
	}, nil
}

func (server *WorkerServer) Serve() error {
	return server.grpcServer.Serve(server.listener)
}

func (server *WorkerServer) Close() error {
	err := server.listener.Close()
	server.service.Cleanup()

	return err
}

func serverConfig() []grpc.ServerOption {
	opts := make([]grpc.ServerOption, 0)

	keepalivePolicy := keepalive.ServerParameters{
		Time:    60 * time.Second,
		Timeout: 180 * time.Second,
	}

	enforcementPolicy := keepalive.EnforcementPolicy{
		MinTime:             60 * time.Second,
		PermitWithoutStream: true,
	}

	// TODO add TLS and Interceptors
	opts = append(opts, grpc.KeepaliveParams(keepalivePolicy))
	opts = append(opts, grpc.KeepaliveEnforcementPolicy(enforcementPolicy))
	opts = append(opts, grpc.MaxConcurrentStreams(1000))

	return opts
}
