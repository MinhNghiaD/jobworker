package job

import (
	"fmt"
	"sync"

	exec "golang.org/x/sys/execabs"

	"github.com/MinhNghiaD/jobworker/pkg/log"
	"github.com/google/uuid"
)

// Jobs manager is the interface for the externals to create and access to job
type JobsManager interface {
	// Create a new job and Start running it in the background with the command and its arguments, then return its ID
	CreateJob(command string, args []string) (string, error)
	// Return a created job corresponding to the ID, if the job is not existed, return nil
	GetJob(ID string) (Job, bool)
	// Perform clean up
	Cleanup() error
}

// the implementation of the Jobs manager
type JobsManagerImpl struct {
	jobStore    map[string]Job
	logsManager *log.LogsManager
	mutex       sync.RWMutex
}

// Create a new Job Manager with its dedicated space
func NewManager() (JobsManager, error) {
	logsManager, err := log.NewManager()

	if err != nil {
		return nil, err
	}

	return &JobsManagerImpl{
		jobStore:    make(map[string]Job),
		logsManager: logsManager,
	}, nil
}

// Start a new job in the background with the command and its arguments
func (manager *JobsManagerImpl) CreateJob(command string, args []string) (string, error) {
	if manager.logsManager == nil {
		return "", fmt.Errorf("Log Manager is not iniatiated")
	}

	if _, err := exec.LookPath(command); err != nil {
		return "", err
	}

	cmd := exec.Command(command, args...)
	ID := uuid.New()
	logger, err := manager.logsManager.NewLogger(ID.String())

	if err != nil {
		return "", err
	}

	j, err := newJob(ID, cmd, logger)

	if err != nil {
		return "", err
	}

	// add job to the store
	manager.mutex.Lock()
	manager.jobStore[ID.String()] = j
	manager.mutex.Unlock()

	// Start job
	err = j.Start()

	return j.ID(), err
}

// Search for the job with the corresponding ID
func (manager *JobsManagerImpl) GetJob(ID string) (Job, bool) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	j, ok := manager.jobStore[ID]

	return j, ok
}

// Cleanup the resources used by the Jobs manager
func (manager *JobsManagerImpl) Cleanup() error {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	for _, j := range manager.jobStore {
		j.Stop(true)
	}

	return manager.logsManager.Cleanup()
}
