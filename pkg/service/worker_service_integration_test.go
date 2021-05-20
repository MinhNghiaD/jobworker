package service_test

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/MinhNghiaD/jobworker/api/worker/proto"
	"github.com/sirupsen/logrus"
)

func TestSimulation(t *testing.T) {
	server, listener := newTestServer()

	go server.Serve(listener)
	defer listener.Close()

	testcases := []struct {
		name string
		cmd  string
		args []string
	}{
		{
			"Empty Command",
			"",
			[]string{},
		},
		{
			"Non exist command",
			"abc",
			[]string{},
		},
		{
			"Short term",
			"ls",
			[]string{"-la"},
		},
		{
			"File access",
			"mkdir",
			[]string{fmt.Sprintf("/tmp/%s", randomString(3))},
		},
		{
			"Check user",
			"whoami",
			[]string{},
		},
		{
			"long running",
			"top",
			[]string{"-b"},
		},
		{
			"High priviledge",
			"apt",
			[]string{"update"},
		},
		{
			"sudo",
			"sudo",
			[]string{"apt", "update"},
		},
		{
			"mask signals force stop",
			"bash",
			[]string{"-c", "trap -- '' SIGINT SIGTERM SIGKILL; while true; do date +%F_%T; sleep 1; done"},
		},
	}

	// Created jobs
	jobIDs := SyncJobSlice{
		jobIDs: make([]string, 0),
	}

	var wg sync.WaitGroup

	// Simulate client random access
	for i := 0; i < 100; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			clientConn := newTestClientConn()
			defer clientConn.Close()
			client := proto.NewWorkerServiceClient(clientConn)

			for j := 0; j < 50; j++ {
				switch rand.Int() % 3 {
				case 0:
					// Start job
					testcase := testcases[rand.Int()%len(testcases)]

					cmd := &proto.Command{
						Cmd:  testcase.cmd,
						Args: testcase.args,
					}

					job, err := client.StartJob(context.Background(), cmd)

					if err != nil {
						logrus.Warningf("Fail to start job, %s", err)
					} else {
						jobIDs.Append(job.Id)
					}
				case 1:
					// Stop job
					request := &proto.StopRequest{
						Job: &proto.Job{
							Id: jobIDs.RandomID(),
						},
						Force: false,
					}

					status, err := client.StopJob(context.Background(), request)
					if err != nil {
						logrus.Warningf("Fail to stop job, %s", err)
					} else {
						logrus.Infof("Stop job, status %s", status)
					}
				case 2:
					// Query job
					job := &proto.Job{
						Id: jobIDs.RandomID(),
					}

					status, err := client.QueryJob(context.Background(), job)
					if err != nil {
						logrus.Warningf("Fail to query job, %s", err)
					} else {
						logrus.Infof("Query job, status %s", status)
					}
				}
			}
		}()
	}

	wg.Wait()
}

type SyncJobSlice struct {
	jobIDs []string
	mutex  sync.RWMutex
}

func (slice *SyncJobSlice) Append(jobID string) {
	slice.mutex.Lock()
	defer slice.mutex.Unlock()

	slice.jobIDs = append(slice.jobIDs, jobID)
}

func (slice *SyncJobSlice) RandomID() string {
	slice.mutex.RLock()
	defer slice.mutex.RUnlock()

	if len(slice.jobIDs) == 0 {
		return ""
	}

	return slice.jobIDs[rand.Int()%len(slice.jobIDs)]
}
