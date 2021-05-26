package auth

import (
	"context"
	"crypto/x509"
	"fmt"

	token "github.com/libopenstorage/openstorage-sdk-auth/pkg/auth"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type AccessesMap map[string](map[string]AccessRight)

type AccessRight struct {
	accessAll bool
}

// TODO: use yaml file to configure RBAC instead of hard coding
var (
	// Default roles. Identified by the 'system' prefix to avoid collisions
	defaultRoles = AccessesMap{
		// system.admin role can access to any APIs
		"system.admin": map[string]AccessRight{
			"/proto.WorkerService/StartJob": {
				accessAll: true,
			},
			"/proto.WorkerService/StopJob": {
				accessAll: true,
			},
			"/proto.WorkerService/QueryJob": {
				accessAll: true,
			},
			"/proto.WorkerService/StreamLog": {
				accessAll: true,
			},
		},
		// system.user role can access to Start/Query any jobs but can only stop or stream their created jobs
		"system.user": map[string]AccessRight{
			"/proto.WorkerService/StartJob": {
				accessAll: true,
			},
			"/proto.WorkerService/StopJob": {
				accessAll: false,
			},
			"/proto.WorkerService/QueryJob": {
				accessAll: true,
			},
			"/proto.WorkerService/StreamLog": {
				accessAll: false,
			},
		},
		// system.observer role can only Query jobs
		"system.observer": map[string]AccessRight{
			"/proto.WorkerService/QueryJob": {
				accessAll: true,
			},
		},
	}
)

// RoleManager manages RBAC authorization and gRPC interceptors of Job worker service
// It intercepts the incomming messages and using the JWT signing certificate to verify user access right to certain APIs
type RoleManager struct {
	accessesMap AccessesMap
	jwtCert     *x509.Certificate
}

// NewRoleManager creates a new RoleManager with its signing certificate
func NewRoleManager(cert *x509.Certificate) *RoleManager {
	return &RoleManager{
		accessesMap: defaultRoles,
		jwtCert:     cert,
	}
}

// AuthorizationUnaryInterceptor intercepts incoming Unary RPC and uses the JWT of the client to perform access control
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

// AuthorizationStreamInterceptor intercepts incoming Streaming RPC and uses the JWT of the client to perform access control
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

// authorize verifies authorization of the token given by the client in its context
func (manager *RoleManager) authorize(ctx context.Context, api string) (context.Context, error) {
	claims, err := getClaimFromContext(ctx, manager.jwtCert)
	if err != nil {
		logrus.Warnf("Refuse user, %s", err.Error())
		return nil, err
	}

	right, err := manager.lookupAccessRight(claims, api)
	if err != nil {
		logrus.Warnf("Refuse user %s request to API %s, %s", claims.Subject, api, err.Error())
		return nil, status.Errorf(codes.PermissionDenied, err.Error())
	}

	// Append user metatdata to the context for jobs access right
	md := metadata.New(map[string]string{
		"user":        claims.Subject,
		"accessright": fmt.Sprintf("%v", right.accessAll),
	})

	return metadata.NewIncomingContext(ctx, md), nil
}

// lookupAccessRight using the claims decoded from the token to verify its access right to the requested API
func (manager *RoleManager) lookupAccessRight(claims *token.Claims, api string) (*AccessRight, error) {
	for _, role := range claims.Roles {
		if accesses, ok := manager.accessesMap[role]; ok {
			if right, ok := accesses[api]; ok {
				return &right, nil
			}
		}
	}

	return nil, fmt.Errorf("User don't have claim to API %s", api)
}

// getClaimFromContext extracts user claims from the gRPC context
func getClaimFromContext(ctx context.Context, certificate *x509.Certificate) (*token.Claims, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.PermissionDenied, "token is not provided")
	}

	if len(md["token"]) == 0 {
		return nil, status.Errorf(codes.PermissionDenied, "access token is not provided")
	}

	return DecodeToken(md["token"][0], certificate)
}

// ContextWrappedServerStream is a wrapper for the real grpc.ServerStream.
// This wrapper allows us to customize the context of stream and inject user information to context metadata.
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
