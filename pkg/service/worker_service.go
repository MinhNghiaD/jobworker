package service

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/MinhNghiaD/jobworker/api/worker/proto"
	"github.com/MinhNghiaD/jobworker/pkg/auth"
	"github.com/MinhNghiaD/jobworker/pkg/job"
	"github.com/sirupsen/logrus"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

// TODO: using grpc compression to reduce message size

// WorkerServer is gRPC Server of worker service
type WorkerServer struct {
	proto.UnimplementedWorkerServiceServer
	grpcServer  *grpc.Server
	listener    net.Listener
	jobsManager job.JobsManager
	authorizer  *auth.RoleManager
	tlsConfig   *tls.Config
}

// NewServer creates a new server, listening at the specified port
func NewServer(port int, tlsConfig *tls.Config) (*WorkerServer, error) {
	logrus.Infof("Listen at port %d", port)
	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return nil, err
	}

	opts := serverConfig()
	if tlsConfig != nil {
		logrus.Info("Start server with TLS authentication")
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsConfig)))
	}

	jobsManager, err := job.NewManager()
	if err != nil {
		return nil, err
	}

	// Add authorization control
	authorizer := auth.NewRoleManager(jobsManager)
	opts = append(opts, grpc.UnaryInterceptor(authorizer.AuthorizationUnaryInterceptor))
	opts = append(opts, grpc.StreamInterceptor(authorizer.AuthorizationStreamInterceptor))

	return &WorkerServer{
		grpcServer:  grpc.NewServer(opts...),
		listener:    listener,
		jobsManager: jobsManager,
		authorizer:  authorizer,
		tlsConfig:   tlsConfig,
	}, nil
}

// Serve tarts the server
func (server *WorkerServer) Serve() error {
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
	// TODO Get Owner common name from certificate
	jobID, err := server.jobsManager.CreateJob(cmd.Cmd, cmd.Args, "User CN")
	if err != nil {
		return nil, err
	}

	return &proto.Job{
		Id: jobID,
	}, nil
}

// StopJob terminates a job specified by user request.
// It returns an error when the internal service is not ready, the job id is not valid or the stop is failed to stop
func (server *WorkerServer) StopJob(ctx context.Context, request *proto.StopRequest) (*proto.JobStatus, error) {
	j, ok := server.jobsManager.GetJob(request.Job.Id)
	if !ok {
		badRequest := &errdetails.BadRequest{
			FieldViolations: []*errdetails.BadRequest_FieldViolation{
				{
					Field:       "job",
					Description: "Job ID is not correct",
				},
			},
		}

		return nil, job.ReportError(codes.NotFound, "Fail to stop job", badRequest)
	}

	if err := j.Stop(request.Force); err != nil {
		return nil, err
	}

	return j.Status(), nil
}

// QueryJob returns the status of a job. It returns an error in case the internal service is not ready or the job id is not valid
func (server *WorkerServer) QueryJob(ctx context.Context, protoJob *proto.Job) (*proto.JobStatus, error) {
	j, ok := server.jobsManager.GetJob(protoJob.Id)
	if !ok {
		badRequest := &errdetails.BadRequest{
			FieldViolations: []*errdetails.BadRequest_FieldViolation{
				{
					Field:       "job",
					Description: "Job ID is not correct",
				},
			},
		}

		return nil, job.ReportError(codes.NotFound, "Fail to query job", badRequest)
	}

	return j.Status(), nil
}

// Tradeoff: For now we are using Server-side streaming to send log in real-time to the client.
// This solution is simple but it can easily overload the server.
// TODO: A better approach is to use bi-directional streaming implement congestion control to negotiate the data flow in ASYNC mode

// StreamLog maintains a stream of job logs specified by user.
// It receives user request, which indicates the job that they want to retrieve the log, and the nb of sequence that the stream should start from.
// The starting point is typically used for connection backoff
func (server *WorkerServer) StreamLog(request *proto.StreamRequest, stream proto.WorkerService_StreamLogServer) error {
	if server.jobsManager == nil {
		return status.Errorf(codes.Unavailable, "Job Managers is not ready")
	}

	j, ok := server.jobsManager.GetJob(request.Job.Id)
	if !ok {
		badRequest := &errdetails.BadRequest{
			FieldViolations: []*errdetails.BadRequest_FieldViolation{
				{
					Field:       "job",
					Description: "Job ID is not correct",
				},
			},
		}

		return job.ReportError(codes.NotFound, "Fail to stream log", badRequest)
	}

	ctx := stream.Context()
	logReader, err := j.GetLogReader(ctx)
	if err != nil {
		return err
	}

	defer logReader.Close()

	// Service will start the stream at the request starting point
	sequenceCounter := request.StartPoint
	for err == nil {
		var line string
		line, err = logReader.ReadAt(int(sequenceCounter))
		if err != nil {
			break
		}

		err = stream.Send(&proto.Log{Entry: line, NbSequence: sequenceCounter})
		sequenceCounter++
	}

	if err != io.EOF {
		return err
	}

	return nil
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

	opts = append(opts, grpc.KeepaliveParams(keepalivePolicy))
	opts = append(opts, grpc.KeepaliveEnforcementPolicy(enforcementPolicy))
	opts = append(opts, grpc.MaxConcurrentStreams(1000))

	return opts
}
