package job

import "fmt"

type JobsManager interface {
	AddJob(command string, args []string) (Job, error)
	GetJob(ID string) (Job, error)
	Cleanup() error
}

type JobsManagerImpl struct {
}

func NewManager() (JobsManager, error) {
	return &JobsManagerImpl{}, nil
}

func (manager *JobsManagerImpl) AddJob(command string, args []string) (Job, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (manager *JobsManagerImpl) GetJob(ID string) (Job, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (manager *JobsManagerImpl) Cleanup() error {
	return fmt.Errorf("Not implemented")
}
