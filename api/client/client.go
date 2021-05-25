package client

import (
	"context"
	"crypto/tls"
	"io"
	"time"

	"github.com/MinhNghiaD/jobworker/api/worker/proto"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

// Client is gRPC Client that allows user to access to Worker APIs
type Client struct {
	connection *grpc.ClientConn
	stub       proto.WorkerServiceClient
	tlsConfig  *tls.Config
}

func NewWithTLS(address string, tlsConfig *tls.Config) (*Client, error) {
	dialOptions := clientDialOptions()
	dialOptions = append(dialOptions, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))

	dialContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	connection, err := grpc.DialContext(dialContext, address, dialOptions...)
	if err != nil {
		return nil, err
	}

	return &Client{
		connection: connection,
		stub:       proto.NewWorkerServiceClient(connection),
		tlsConfig:  tlsConfig,
	}, nil
}

// NewWithInsecure creates a new Client to connect to server address specified in the parameters
func NewWithInsecure(address string) (*Client, error) {
	dialOptions := clientDialOptions()
	dialOptions = append(dialOptions, grpc.WithInsecure())

	dialContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	connection, err := grpc.DialContext(dialContext, address, dialOptions...)
	if err != nil {
		return nil, err
	}

	return &Client{
		connection: connection,
		stub:       proto.NewWorkerServiceClient(connection),
	}, nil
}

// Close closes the gRpc connection
func (c *Client) Close() error {
	return c.connection.Close()
}

// StartJob starts a job on the server
func (c *Client) StartJob(ctx context.Context, cmd *proto.Command) (*proto.Job, error) {
	return c.stub.StartJob(ctx, cmd)
}

// QueryJob queries the status of a job on the server
func (c *Client) QueryJob(ctx context.Context, j *proto.Job) (*proto.JobStatus, error) {
	return c.stub.QueryJob(ctx, j)
}

// StopJob stop a job on the server and return its status
func (c *Client) StopJob(ctx context.Context, request *proto.StopRequest) (*proto.JobStatus, error) {
	return c.stub.StopJob(ctx, request)
}

// GetLogReceiver returns a LogReceiver that can read log stream line by line
func (c *Client) GetLogReceiver(ctx context.Context, j *proto.Job) (*LogReceiver, error) {
	stream, err := c.stub.StreamLog(ctx, &proto.StreamRequest{
		Job:        j,
		StartPoint: 0,
	})

	if err != nil {
		return nil, err
	}

	return &LogReceiver{
		cli:            c,
		stream:         stream,
		latestSequence: -1,
		job:            j,
		ctx:            ctx,
	}, nil
}

// LogReceiver handle a log stream from the server. LogReceiver is not designed to be thread-safe
type LogReceiver struct {
	cli    *Client
	stream proto.WorkerService_StreamLogClient
	job    *proto.Job
	ctx    context.Context
	// latestSequence indicate the latest log sequence received by the receiver
	// it is used in reset in order to continue the current stream, instead of restart from the beginning
	latestSequence int32
}

// reset resets client connection and request a new stream start from the current sequence
func (receiver *LogReceiver) reset() error {
	receiver.cli.connection.ResetConnectBackoff()

	var err error
	receiver.stream, err = receiver.cli.stub.StreamLog(receiver.ctx, &proto.StreamRequest{
		Job: receiver.job,
		// point the start point to the next sequence
		StartPoint: receiver.latestSequence + 1,
	})

	return err
}

// Read reads the next log entry. Read implement connection backoff to handle connection error.
// When a connection error occurs, the client will try to reset the connection and resume the stream instead of restart from the beginning.
// The maximum retry period is 1 minute. After 1 minute, if the connection is still not fixed, Read returns the corresponding connection error.
func (receiver *LogReceiver) Read() (line *proto.Log, err error) {
	line = nil
	const maxBackoff = time.Minute
	currentBackoff := time.Second

	// Attempt to read a log entry, with a retry period of maxBackoff
	for line == nil && currentBackoff < maxBackoff {
		if receiver.stream != nil {
			line, err = receiver.stream.Recv()
			if err != nil {
				if err == io.EOF {
					return nil, err
				}

				// Decode error code, we allow backoff only when the error is Unavailable, DeadlineExceeded
				logrus.Debugf("Fail to receive data, %s", err)
				errCode := status.Convert(err).Code()
				if errCode != codes.Unavailable && errCode != codes.DeadlineExceeded {
					return nil, err
				}

				// Clearing the stream will force the client to resubscribe on next iteration
				receiver.stream = nil
			} else {
				// Update the sequence number
				receiver.latestSequence = line.NbSequence
				break
			}
		}

		if receiver.stream == nil {
			select {
			case <-time.After(currentBackoff):
				currentBackoff *= 2
			case <-receiver.ctx.Done():
				return nil, status.Error(codes.DeadlineExceeded, "Cancelled")
			}

			if e := receiver.reset(); e != nil {
				logrus.Debugf("Fail to reset stream, %s", e)
			}
		}
	}

	return line, err
}

// clientDialOptions returns the gRPC configurations of the connection
func clientDialOptions() []grpc.DialOption {
	opts := make([]grpc.DialOption, 0)

	// TODO configure TLS
	keepalivePolicy := keepalive.ClientParameters{
		Time:    60 * time.Second,
		Timeout: 180 * time.Second,
	}

	connectParameters := grpc.ConnectParams{
		Backoff: backoff.Config{
			BaseDelay:  time.Second,
			Multiplier: 1.5,
			Jitter:     0.2,
			MaxDelay:   120 * time.Second,
		},
		MinConnectTimeout: 20 * time.Second,
	}

	opts = append(opts, grpc.WithKeepaliveParams(keepalivePolicy))
	opts = append(opts, grpc.WithConnectParams(connectParameters))

	return opts
}
