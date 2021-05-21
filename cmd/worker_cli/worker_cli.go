package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MinhNghiaD/jobworker/api/client"
	"github.com/MinhNghiaD/jobworker/api/worker/proto"
	"github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	address = kingpin.Flag("a", "server address").Default("127.0.0.1:7777").String()
	// TODO add flags for certificate, private key and server CAs

	start    = kingpin.Command("start", "Start a job on worker service.")
	startCmd = start.Flag("cmd", "command to be executed").Default("").String()

	stop        = kingpin.Command("stop", "Stop a job on worker service.")
	stopForce   = stop.Flag("force", "force job to terminate immediately").Default("false").Bool()
	stoppingJob = stop.Flag("job", "job id").Default("").String()

	query      = kingpin.Command("query", "Query status of a job on worker service.")
	queriedJob = query.Flag("job", "job id").Default("").String()
)

func main() {

	subCommand := kingpin.Parse()

	cli, err := client.New(*address)

	if err != nil {
		logrus.Fatalf("Fail to init client %s", err)
	}

	defer cli.Close()

	switch subCommand {
	case "start":
		startJob(cli, *startCmd)
	case "stop":
		stopJob(cli, *stoppingJob, *stopForce)
	case "query":
		queryJob(cli, *queriedJob)
	}

}

func startJob(c *client.Client, cmd string) {
	if c == nil {
		logrus.Error("Client is not initiated")
		return
	}

	fields := strings.Fields(cmd)
	command := &proto.Command{
		Cmd:  fields[0],
		Args: fields[1:],
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	j, err := c.Stub.StartJob(ctx, command)

	if err != nil {
		logrus.Errorf("Fail to start job, %s", err)
		return
	}

	fmt.Printf("Start job sucessfully, job ID: %s\n", j.Id)
}

func stopJob(c *client.Client, jobID string, force bool) {
	if c == nil {
		logrus.Error("Client is not initiated")
		return
	}

	request := &proto.StopRequest{
		Job:   &proto.Job{Id: jobID},
		Force: force,
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	status, err := c.Stub.StopJob(ctx, request)

	if err != nil {
		logrus.Errorf("Fail to stop job, %s", err)
		return
	}

	printJobStatus(status)
}

func queryJob(c *client.Client, jobID string) {
	if c == nil {
		logrus.Error("Client is not initiated")
		return
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	status, err := c.Stub.QueryJob(ctx, &proto.Job{Id: jobID})

	if err != nil {
		logrus.Errorf("Fail to query job, %s", err)
		return
	}

	printJobStatus(status)
}

func printJobStatus(status *proto.JobStatus) {
	fmt.Printf("Job %s: \n", status.GetJob().GetId())
	fmt.Printf("\t - Command: %s \n", status.GetCommand().GetCmd())
	fmt.Printf("\t - Creater: %s \n", status.GetOwner())
	fmt.Printf("\t - Status : \n")
	fmt.Printf("\t\t - PID      : %d \n", status.GetStatus().Pid)
	fmt.Printf("\t\t - State    : %v \n", status.GetStatus().State)
	fmt.Printf("\t\t - Exit code: %d \n", status.GetStatus().ExitCode)
}
