package service_test

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/MinhNghiaD/jobworker/api/client"
	"github.com/MinhNghiaD/jobworker/api/worker/proto"
	"github.com/MinhNghiaD/jobworker/pkg/service"
	"github.com/sirupsen/logrus"
)

// TestSimulation runs simulation of user requests to the worker service.
// It enable an observation to scan the functionalities as a whole,
// rather than focus on the detail functionalities, which are covered in other independent tests.
// Data race, Goroutine leak, deadlock and system crash can be detected by this test
func TestSimulation(t *testing.T) {
	rand.Seed(1420)
	server, err := service.NewServer(7777)
	if err != nil {
		t.Fatal(err)
	}

	go server.Serve()
	defer server.Close()

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
			"High privilege",
			"apt",
			[]string{"update"},
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
	for i := 0; i < 10; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			client, err := client.NewWithInsecure("127.0.0.1:7777")
			if err != nil {
				t.Errorf("Fail to init client %s", err)
				return
			}

			defer client.Close()

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

			// Stream log for 5 seconds after finished all uniary operations
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			receiver, err := client.GetLogReceiver(ctx, &proto.Job{Id: jobIDs.RandomID()})
			for err == nil {
				_, err = receiver.Read()
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
