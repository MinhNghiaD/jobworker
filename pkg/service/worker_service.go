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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
)

// WorkerServer is gRPC Server of worker service
type WorkerServer struct {
	proto.UnimplementedWorkerServiceServer
	opts        []grpc.ServerOption
	grpcServer  *grpc.Server
	listener    net.Listener
	jobsManager job.JobsManager
}

// NewServer creates a new server, listening at the specified port
func NewServer(port int) (*WorkerServer, error) {
	logrus.Infof("Listen at port %d", port)
	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return nil, err
	}

	opts := serverConfig()
	jobsManager, err := job.NewManager()
	if err != nil {
		return nil, err
	}

	return &WorkerServer{
		opts:        opts,
		grpcServer:  nil,
		listener:    listener,
		jobsManager: jobsManager,
	}, nil
}

// Serve tarts the server
func (server *WorkerServer) Serve() error {
	server.grpcServer = grpc.NewServer(server.opts...)
	proto.RegisterWorkerServiceServer(server.grpcServer, server)

	return server.grpcServer.Serve(server.listener)
}

// Close closes the listener and perform cleanup
func (server *WorkerServer) Close() error {
	server.grpcServer.GracefulStop()
	err := server.listener.Close()
	server.jobsManager.Cleanup()

	return err
}

// StartJob starts a job corresponding to user request
func (server *WorkerServer) StartJob(ctx context.Context, cmd *proto.Command) (*proto.Job, error) {
	jobID, err := server.jobsManager.CreateJob(cmd.Cmd, cmd.Args, "User CN")
	if err != nil {
		return nil, err
	}

	return &proto.Job{
		Id: jobID,
	}, nil
}

// StopJob terminates a job specified by user request
func (server *WorkerServer) StopJob(ctx context.Context, request *proto.StopRequest) (*proto.JobStatus, error) {
	j, ok := server.jobsManager.GetJob(request.Job.Id)
	if !ok {
		return nil, job.ReportError(codes.NotFound, "Fail to stop job", "job", "Job ID is not correct")
	}

	if err := j.Stop(request.Force); err != nil {
		return nil, err
	}

	return j.Status(), nil
}

// QueryJob returns the status of a job
func (server *WorkerServer) QueryJob(ctx context.Context, protoJob *proto.Job) (*proto.JobStatus, error) {
	j, ok := server.jobsManager.GetJob(protoJob.Id)
	if !ok {
		return nil, job.ReportError(codes.NotFound, "Fail to query job", "job", "Job ID is not correct")
	}

	return j.Status(), nil
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
