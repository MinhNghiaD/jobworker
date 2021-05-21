package client

import (
	"time"

	"github.com/MinhNghiaD/jobworker/api/worker/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

// Client is gRPC Client that allows user to access to Worker APIs
type Client struct {
	connection *grpc.ClientConn
	Stub       proto.WorkerServiceClient
}

// New creates a new Client to connect to server address specified in the parameters
func New(address string) (*Client, error) {
	opts := clientconfig()

	connection, err := grpc.Dial(address, opts...)

	if err != nil {
		return nil, err
	}

	Stub := proto.NewWorkerServiceClient(connection)

	return &Client{
		connection: connection,
		Stub:       Stub,
	}, nil
}

// Close closes the gRpc connection
func (c *Client) Close() error {
	return c.connection.Close()
}

// clientconfig returns the gRPC configuration of the connection
func clientconfig() []grpc.DialOption {
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
