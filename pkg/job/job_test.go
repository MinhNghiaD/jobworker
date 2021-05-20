package job_test

import (
	"testing"

	"github.com/MinhNghiaD/jobworker/pkg/job"
)

func TestStartJobs(t *testing.T) {
	manager, err := job.NewManager()

	if err != nil {
		t.Fatal(err)
	}

	// checker
	checkStartCmd := func(cmd string, args []string) error {
		j, err := manager.AddJob(cmd, args)

		if err != nil {
			return err
		}

		return j.Start()
	}

	testcases := []struct {
		name      string
		cmd       string
		args      []string
		expectErr bool
	}{
		{
			"Empty Command",
			"",
			[]string{},
			true,
		},
		{
			"Non exist command",
			"abc",
			[]string{},
			true,
		},
		{
			"ls",
			"ls",
			[]string{"-la"},
			false,
		},
		{
			"mkdir",
			"mkdir",
			[]string{"/tmp/testdir"},
			false,
		},
		{
			"whoami",
			"whoami",
			[]string{},
			false,
		},
		{
			"top",
			"top",
			[]string{"-c"},
			false,
		},
		{
			"High priviledge",
			"apt",
			[]string{"update"},
			false,
		},
		{
			"sudo",
			"sudo",
			[]string{"apt", "update"},
			false,
		},
	}

	for _, testCase := range testcases {
		t.Run(testCase.name, func(t *testing.T) {
			err := checkStartCmd(testCase.cmd, testCase.args)

			if (err != nil) != (testCase.expectErr) {
				t.Errorf("Test case %s with input %s returns error %v, while %v error is expected", testCase.name, testCase.cmd, err, testCase.expectErr)
				return
			}
		})
	}
}
