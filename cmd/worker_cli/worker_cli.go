package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/MinhNghiaD/jobworker/api/client"
	"github.com/MinhNghiaD/jobworker/api/worker/proto"
	"github.com/sirupsen/logrus"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/status"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	address = kingpin.Flag("a", "server address").Default("127.0.0.1:7777").String()
	// TODO add flags for certificate, private key and server CAs

	// Start subcommand and its flags
	start     = kingpin.Command("start", "Start a job on worker service.")
	startCmd  = start.Flag("cmd", "command to be executed").Required().String()
	startArgs = start.Arg("args", "arguments of the command").Strings()

	// Stop subcommand and its flags
	stop        = kingpin.Command("stop", "Stop a job on worker service.")
	stopForce   = stop.Flag("force", "force job to terminate immediately").Default("false").Bool()
	stoppingJob = stop.Flag("job", "job id").Required().String()

	// Query subcommand and its flags
	query      = kingpin.Command("query", "Query status of a job on worker service.")
	queriedJob = query.Flag("job", "job id").Required().String()

	// Stream subcommand and its flags
	stream    = kingpin.Command("stream", "Stream log of a job on worker service.")
	streamJob = stream.Flag("job", "job id").Required().String()
)

func main() {
	subCommand := kingpin.Parse()

	cli, err := client.NewWithInsecure(*address)
	if err != nil {
		logrus.Fatalf("Fail to init client %s", err)
	}
	defer cli.Close()

	switch subCommand {
	case start.FullCommand():
		err = startJob(cli, *startCmd, *startArgs)
	case stop.FullCommand():
		err = stopJob(cli, *stoppingJob, *stopForce)
	case query.FullCommand():
		err = queryJob(cli, *queriedJob)
	case stream.FullCommand():
		err = streamLog(cli, *streamJob)
	default:
		logrus.Fatal("Unknown subcommand")
	}

	if err != nil {
		reportError(err)
		logrus.Exit(1)
	}
}

// startJob using the gRPC client to start a job with the correspodning command on the server
func startJob(c *client.Client, cmd string, args []string) error {
	command := &proto.Command{
		Cmd:  cmd,
		Args: args,
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	j, err := c.StartJob(ctx, command)
	if err != nil {
		return err
	}

	fmt.Printf("Start job successfully, job ID: %s\n", j.Id)
	return nil
}

// stopJob stops the corresponding job with force option
func stopJob(c *client.Client, jobID string, force bool) error {
	request := &proto.StopRequest{
		Job:   &proto.Job{Id: jobID},
		Force: force,
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	jobStatus, err := c.StopJob(ctx, request)
	if err != nil {
		return err
	}

	printJobStatus(jobStatus)
	return nil
}

// queryJob queries the status of a job specified by jobID
func queryJob(c *client.Client, jobID string) error {
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	jobStatus, err := c.QueryJob(ctx, &proto.Job{Id: jobID})
	if err != nil {
		return err
	}

	printJobStatus(jobStatus)
	return nil
}

// queryJob queries the status of a job specified by jobID
func streamLog(c *client.Client, jobID string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	receiver, err := c.GetLogReceiver(ctx, &proto.Job{Id: jobID})
	if err != nil {
		return err
	}

	for {
		in, err := receiver.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		var data map[string]string
		if err := json.Unmarshal([]byte(in.Entry), &data); err != nil {
			logrus.Warn("Fail to decode json format")
		} else {
			fmt.Printf("%s: %s\n", data["source"], data["msg"])
		}
	}
}

// printJobStatus displays the job status
func printJobStatus(jobStatus *proto.JobStatus) {
	fmt.Printf("Job %s: \n", jobStatus.GetJob().GetId())
	fmt.Printf("\t - Command: %s \n", jobStatus.GetCommand().GetCmd())
	fmt.Printf("\t - Creator: %s \n", jobStatus.GetOwner())
	fmt.Printf("\t - Status : \n")
	fmt.Printf("\t\t - PID      : %d \n", jobStatus.GetStatus().Pid)
	fmt.Printf("\t\t - State    : %v \n", jobStatus.GetStatus().State)
	fmt.Printf("\t\t - Exit code: %d \n", jobStatus.GetStatus().ExitCode)
}

func reportError(err error) {
	s := status.Convert(err)
	logrus.Errorf("Fail to execute RPC, error code %s, description %s", s.Code(), s.Message())

	for _, d := range s.Details() {
		switch info := d.(type) {
		case *errdetails.QuotaFailure:
			logrus.Errorf("Quota failure: %s", info)
		default:
			logrus.Errorf("Unexpected error: %s", info)
		}
	}
}
