package auth

import (
	"context"
	"log"

	"google.golang.org/grpc"
)

func AuthorizationUnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	log.Println("--> unary interceptor: ", info.FullMethod)
	return handler(ctx, req)
}

func AuthorizationStreamInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	log.Println("--> stream interceptor: ", info.FullMethod)
	return handler(srv, stream)
}
