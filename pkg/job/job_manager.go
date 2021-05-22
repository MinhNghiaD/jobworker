package job

import (
	"fmt"
	"os/exec"
	"sync"

	"github.com/MinhNghiaD/jobworker/pkg/log"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// JobsManager is the interface for the externals to create and access to job
type JobsManager interface {
	// Create a new job and Start running it in the background with the command and its arguments, then return its ID
	CreateJob(command string, args []string, owner string) (string, error)
	// Return a created job corresponding to the ID, if the job is not existed, return nil
	GetJob(ID string) (Job, bool)
	// Perform clean up
	Cleanup() error
}

// JobsManagerImpl is the implementation of the Jobs manager
type JobsManagerImpl struct {
	jobStore    map[string]Job
	logsManager *log.LogsManager
	mutex       sync.RWMutex
}

// NewManager creates a new Job Manager with its dedicated space
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

// CreateJob starts a new job in the background with the command and its arguments, then associates the job with its owner
func (manager *JobsManagerImpl) CreateJob(command string, args []string, owner string) (string, error) {
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

	j, err := newJob(ID, cmd, logger, owner)
	if err != nil {
		return "", err
	}

	// add job to the store
	manager.mutex.Lock()
	manager.jobStore[ID.String()] = j
	manager.mutex.Unlock()

	// Start job
	err = j.Start()

	return ID.String(), err
}

// GetJob searches for the job with the corresponding ID
func (manager *JobsManagerImpl) GetJob(ID string) (Job, bool) {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	j, ok := manager.jobStore[ID]

	return j, ok
}

// Cleanup cleanups the resources and log files used by the Jobs manager
func (manager *JobsManagerImpl) Cleanup() error {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	logrus.Infof("Cleanup jobs")

	for _, j := range manager.jobStore {
		if err := j.Stop(true); err != nil && err != ErrNotRunning {
			logrus.Warningf("Fail to stop job, %s", err)
		}
	}

	return manager.logsManager.Cleanup()
}
