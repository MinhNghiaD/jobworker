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

// WorkerService is the gRPC Service of Worker
type WorkerService struct {
	proto.UnimplementedWorkerServiceServer
	jobsManager job.JobsManager
}

// newWorkerService initiates new service
func newWorkerService() (*WorkerService, error) {
	jobsManager, err := job.NewManager()
	if err != nil {
		return nil, err
	}

	return &WorkerService{
		jobsManager: jobsManager,
	}, nil
}

// StartJob starts a job corresponding to user request
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

// StopJob terminates a job specified by user request
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

// QueryJob returns the status of a job
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

// StreamLog maintains a stream of job logs specified by user
func (service *WorkerService) StreamLog(job *proto.Job, stream proto.WorkerService_StreamLogServer) error {
	return fmt.Errorf("Unimplemented")
}

// Cleanup cleanups the service
func (service *WorkerService) Cleanup() error {
	if service.jobsManager == nil {
		return fmt.Errorf("Job Managers is not ready")
	}

	return service.jobsManager.Cleanup()
}

// WorkerServer is gRPC Server of worker service
type WorkerServer struct {
	grpcServer *grpc.Server
	listener   net.Listener
	service    *WorkerService
}

// NewServer creates a new server, listening at the specified port
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

// Serve tarts the server
func (server *WorkerServer) Serve() error {
	return server.grpcServer.Serve(server.listener)
}

// Close closes the listener and perform cleanup
func (server *WorkerServer) Close() error {
	err := server.listener.Close()
	server.service.Cleanup()

	return err
}

// serverConfig returns gRPC Server configurations
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
	//opts = append(opts, grpc.MaxConcurrentStreams(1000))

	return opts
}
