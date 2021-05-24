package client

import (
	"context"
	"io"
	"time"

	"github.com/MinhNghiaD/jobworker/api/worker/proto"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

// Client is gRPC Client that allows user to access to Worker APIs
type Client struct {
	connection *grpc.ClientConn
	stub       proto.WorkerServiceClient
}

// New creates a new Client to connect to server address specified in the parameters
func New(address string) (*Client, error) {
	dialOptions := clientDialOptions()

	dialContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	connection, err := grpc.DialContext(dialContext, address, dialOptions...)
	if err != nil {
		return nil, err
	}

	stub := proto.NewWorkerServiceClient(connection)

	return &Client{
		connection: connection,
		stub:       stub,
	}, nil
}

// Close closes the gRpc connection
func (c *Client) Close() error {
	return c.connection.Close()
}

func (c *Client) StartJob(ctx context.Context, cmd *proto.Command) (*proto.Job, error) {
	return c.stub.StartJob(ctx, cmd)
}

func (c *Client) QueryJob(ctx context.Context, j *proto.Job) (*proto.JobStatus, error) {
	return c.stub.QueryJob(ctx, j)
}

func (c *Client) StopJob(ctx context.Context, request *proto.StopRequest) (*proto.JobStatus, error) {
	return c.stub.StopJob(ctx, request)
}

func (c *Client) GetLogReceiver(ctx context.Context, j *proto.Job) (*LogReceiver, error) {
	stream, err := c.stub.StreamLog(ctx, &proto.StreamRequest{
		Job:        j,
		StartPoint: 0,
	})

	if err != nil {
		return nil, err
	}

	return &LogReceiver{
		stub:           c.stub,
		stream:         stream,
		latestSequence: 0,
		job:            j,
		ctx:            ctx,
	}, nil
}

// LogReceiver handle a log stream from the server. LogReceiver is not designed to be thread-safe
type LogReceiver struct {
	stub           proto.WorkerServiceClient
	stream         proto.WorkerService_StreamLogClient
	latestSequence int32
	job            *proto.Job
	ctx            context.Context
}

func (receiver *LogReceiver) reset() error {
	request := &proto.StreamRequest{
		Job:        receiver.job,
		StartPoint: receiver.latestSequence,
	}

	var err error
	receiver.stream, err = receiver.stub.StreamLog(receiver.ctx, request)

	return err
}

func (receiver *LogReceiver) Read() (line *proto.Log, err error) {
	line = nil
	maxBackoff := time.Minute
	currentBackoff := time.Second

	for line == nil && currentBackoff < maxBackoff {
		if receiver.stream != nil {
			line, err = receiver.stream.Recv()
			if err != nil {
				if err == io.EOF {
					return nil, err
				}

				logrus.Debugf("Fail to receive data, %s", err)

				errCode := status.Convert(err).Code()
				if errCode != codes.Unavailable && errCode != codes.DeadlineExceeded && errCode != codes.DataLoss {
					return nil, err
				}

				// Clearing the stream will force the client to resubscribe on next iteration
				receiver.stream = nil
			} else {
				receiver.latestSequence++
				break
			}
		}

		if receiver.stream == nil {
			// Reset stream
			if e := receiver.reset(); e != nil {
				logrus.Debugf("Fail to reset stream, %s", e)
			}
		}

		logrus.Debugf("Try to reconnect")
		select {
		case <-time.After(currentBackoff):
			currentBackoff *= 2
		case <-receiver.ctx.Done():
			return nil, status.Error(codes.DeadlineExceeded, "Cancelled")
		}
	}

	return line, err
}

// clientDialOptions returns the gRPC configuration of the connection
func clientDialOptions() []grpc.DialOption {
	opts := make([]grpc.DialOption, 0)

	// TODO configure TLS
	keepalivePolicy := keepalive.ClientParameters{
		Time:    60 * time.Second,
		Timeout: 180 * time.Second,
	}

	opts = append(opts, grpc.WithInsecure())
	opts = append(opts, grpc.WithKeepaliveParams(keepalivePolicy))

	return opts
}
