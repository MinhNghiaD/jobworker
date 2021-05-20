package job

import (
	"fmt"
	"sync"

	exec "golang.org/x/sys/execabs"

	"github.com/google/uuid"
)

// Jobs manager is the interface for the externals to create and access to job
type JobsManager interface {
	// Create a new job and Start running it in the background with the command and its arguments
	CreateJob(command string, args []string) (Job, error)
	// Return a created job corresponding to the ID, if the job is not existed, return nil
	GetJob(ID string) (Job, error)
	// Perform clean up
	Cleanup() error
}

// the implementation of the Jobs manager
type JobsManagerImpl struct {
	jobStore map[uuid.UUID]Job
	mutex    sync.RWMutex
}

// Create a new Job Manager with its dedicated space
func NewManager() (JobsManager, error) {
	return &JobsManagerImpl{
		jobStore: make(map[uuid.UUID]Job),
	}, nil
}

// Start a new job in the background with the command and its arguments
func (manager *JobsManagerImpl) CreateJob(command string, args []string) (Job, error) {
	if _, err := exec.LookPath(command); err != nil {
		return nil, err
	}

	cmd := exec.Command(command, args...)
	ID := uuid.New()

	j, err := newJob(ID, cmd)

	if err != nil {
		return nil, err
	}

	// add job to the store
	manager.mutex.Lock()
	manager.jobStore[ID] = j
	manager.mutex.Unlock()

	// Start job
	err = j.Start()

	return j, err
}

// Search for the job with the corresponding ID
func (manager *JobsManagerImpl) GetJob(ID string) (Job, error) {
	return nil, fmt.Errorf("Not implemented")
}

// Cleanup the resources used by the Jobs manager
func (manager *JobsManagerImpl) Cleanup() error {
	return fmt.Errorf("Not implemented")
}
