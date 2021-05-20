package job

import (
	"fmt"
	"sync"

	exec "golang.org/x/sys/execabs"

	"github.com/google/uuid"
)

type JobsManager interface {
	AddJob(command string, args []string) (Job, error)
	GetJob(ID string) (Job, error)
	Cleanup() error
}

type JobsManagerImpl struct {
	jobStore map[uuid.UUID]Job
	mutex    sync.RWMutex
}

func NewManager() (JobsManager, error) {
	return &JobsManagerImpl{
		jobStore: make(map[uuid.UUID]Job),
	}, nil
}

func (manager *JobsManagerImpl) AddJob(command string, args []string) (Job, error) {
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
	defer manager.mutex.Unlock()
	manager.jobStore[ID] = j

	return j, nil
}

func (manager *JobsManagerImpl) GetJob(ID string) (Job, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (manager *JobsManagerImpl) Cleanup() error {
	return fmt.Errorf("Not implemented")
}
