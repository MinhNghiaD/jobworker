package auth

import (
	"context"
	"fmt"
	"log"
	"reflect"

	"github.com/MinhNghiaD/jobworker/api/worker/proto"
	"github.com/MinhNghiaD/jobworker/pkg/job"
	token "github.com/libopenstorage/openstorage-sdk-auth/pkg/auth"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type AccessesMap map[string](map[string]AccessRight)

type AccessRight struct {
	api       string
	accessAll bool
}

// TODO: use config file to configure access rules instead of hard coding
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
}

func NewRoleManager(jobsManager job.JobsManager) *RoleManager {
	return &RoleManager{
		accessesMap: defaultRoles,
		jobsManager: jobsManager,
	}
}

func (manager *RoleManager) verifyAccess(claims *token.Claims, api string, protoJob *proto.Job) bool {
	right, err := manager.lookupAccessRight(claims, api)
	if err != nil {
		logrus.Warnf("User %s doesn't have access to %s", claims.Subject, api)
		return false
	}

	// If user have the claim to access all job via this API the gain the access
	if right.accessAll {
		return true
	}

	if protoJob == nil {
		return false
	}

	// Control if user is the owner of the job
	if j, ok := manager.jobsManager.GetJob(protoJob.Id); !ok || !reflect.DeepEqual(j.Owner(), claims.Subject) {
		return false
	}

	return true
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

func (manager *RoleManager) AuthorizationUnaryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	log.Println("--> unary interceptor: ", info.FullMethod)
	return handler(ctx, req)
}

func (manager *RoleManager) AuthorizationStreamInterceptor(
	srv interface{},
	stream grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	log.Println("--> stream interceptor: ", info.FullMethod)
	return handler(srv, stream)
}
