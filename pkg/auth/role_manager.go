package auth

import (
	"context"
	"crypto/x509"
	"fmt"

	"github.com/MinhNghiaD/jobworker/pkg/job"
	token "github.com/libopenstorage/openstorage-sdk-auth/pkg/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type AccessesMap map[string](map[string]AccessRight)

type AccessRight struct {
	api       string
	accessAll bool
}

// TODO: use yaml file to configure RBAC instead of hard coding
var (
	// Default roles. Identified by the 'system' prefix to avoid collisions
	defaultRoles = AccessesMap{
		// system.admin role can access to any APIs
		"system.admin": map[string]AccessRight{
			"/proto.WorkerService/StartJob": {
				api:       "/proto.WorkerService/StartJob",
				accessAll: true,
			},
			"/proto.WorkerService/StopJob": {
				api:       "/proto.WorkerService/StopJob",
				accessAll: true,
			},
			"/proto.WorkerService/QueryJob": {
				api:       "/proto.WorkerService/QueryJob",
				accessAll: true,
			},
			"/proto.WorkerService/StreamLog": {
				api:       "/proto.WorkerService/StreamLog",
				accessAll: true,
			},
		},
		// system.user role can access to Start/Query any jobs but can only stop or stream their created jobs
		"system.user": map[string]AccessRight{
			"/proto.WorkerService/StartJob": {
				api:       "/proto.WorkerService/StartJob",
				accessAll: true,
			},
			"/proto.WorkerService/StopJob": {
				api:       "/proto.WorkerService/StopJob",
				accessAll: false,
			},
			"/proto.WorkerService/QueryJob": {
				api:       "/proto.WorkerService/QueryJob",
				accessAll: true,
			},
			"/proto.WorkerService/StreamLog": {
				api:       "/proto.WorkerService/StreamLog",
				accessAll: false,
			},
		},
		// system.observer role can only Query jobs
		"system.observer": map[string]AccessRight{
			"/proto.WorkerService/StreamLog": {
				api:       "/proto.WorkerService/QueryJob",
				accessAll: true,
			},
		},
	}
)

type RoleManager struct {
	accessesMap AccessesMap
	jobsManager job.JobsManager
	jwtCert     *x509.Certificate
}

func NewRoleManager(jobsManager job.JobsManager) *RoleManager {
	return &RoleManager{
		accessesMap: defaultRoles,
		jobsManager: jobsManager,
		jwtCert:     nil,
	}
}

func (manager *RoleManager) AuthorizationUnaryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	ctx, err := manager.authorize(ctx, info.FullMethod)
	if err != nil {
		return nil, err
	}

	return handler(ctx, req)
}

func (manager *RoleManager) AuthorizationStreamInterceptor(
	srv interface{},
	stream grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	ctx := stream.Context()

	ctx, err := manager.authorize(ctx, info.FullMethod)
	if err != nil {
		return err
	}

	return handler(srv, &ContextWrappedServerStream{
		inner: stream,
		ctx:   ctx,
	})
}

func (manager *RoleManager) authorize(ctx context.Context, api string) (context.Context, error) {
	claims, err := getClaimFromContext(ctx, manager.jwtCert)
	if err != nil {
		return nil, err
	}

	right, err := manager.lookupAccessRight(claims, api)
	if err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "user doesn't have access to API")
	}

	// Append user metatdata to the context
	md := metadata.New(map[string]string{
		"user":        claims.Subject,
		"accessRight": fmt.Sprintf("%v", right.accessAll),
	})

	if err != nil {
		return nil, err
	}

	return metadata.NewIncomingContext(ctx, md), nil
}

func (manager *RoleManager) lookupAccessRight(claims *token.Claims, api string) (*AccessRight, error) {
	for _, role := range claims.Roles {
		if accesses, ok := manager.accessesMap[role]; ok {
			if right, ok := accesses[api]; ok {
				return &right, nil
			}
		}
	}

	return nil, fmt.Errorf("User don't have claim to this API")
}

func getClaimFromContext(ctx context.Context, certificate *x509.Certificate) (*token.Claims, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.PermissionDenied, "token is not provided")
	}

	tokens, _ := md["token"]
	if len(tokens) == 0 {
		return nil, status.Errorf(codes.PermissionDenied, "access token is not provided")
	}

	return DecodeToken(tokens[0], certificate)
}

// ContextWrappedServerStream is a wrapper for the real grpc.ServerStream. This wrapper is able to customize the context of stream
type ContextWrappedServerStream struct {
	inner grpc.ServerStream
	ctx   context.Context
}

func (stream *ContextWrappedServerStream) RecvMsg(m interface{}) error {
	return stream.inner.RecvMsg(m)
}

func (stream *ContextWrappedServerStream) SendMsg(m interface{}) error {
	return stream.inner.SendMsg(m)
}

func (stream *ContextWrappedServerStream) SetHeader(m metadata.MD) error {
	return stream.inner.SetHeader(m)
}

func (stream *ContextWrappedServerStream) SendHeader(m metadata.MD) error {
	return stream.inner.SendHeader(m)
}

func (stream *ContextWrappedServerStream) SetTrailer(m metadata.MD) {
	stream.inner.SetTrailer(m)
}

func (stream *ContextWrappedServerStream) Context() context.Context {
	return stream.ctx
}
