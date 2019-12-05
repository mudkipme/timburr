package task

import (
	"sync"
)

type TaskExecutor interface {
	Execute(message []byte) error
}

type TaskType int16

const (
	JobRunnerTask TaskType = iota
	PurgeTask
)

var getexMutex sync.Mutex
var taskMap = make(map[TaskType]TaskExecutor)

func (t TaskType) String() string {
	switch t {
	case JobRunnerTask:
		return "job-runner"
	case PurgeTask:
		return "purge"
	}
	return "unknown"
}

func (t TaskType) GetExecutor() TaskExecutor {
	getexMutex.Lock()
	defer getexMutex.Unlock()

	if executor, ok := taskMap[t]; ok {
		return executor
	}

	var executor TaskExecutor
	switch t {
	case JobRunnerTask:
		executor = DefaultJobRunnerExecutor()
		break
	case PurgeTask:
		executor = DefaultPurgeTask()
		break
	}
	taskMap[t] = executor
	return executor
}

func TaskTypeFromString(s string) TaskType {
	switch s {
	case JobRunnerTask.String():
		return JobRunnerTask
	case PurgeTask.String():
		return PurgeTask
	default:
		return JobRunnerTask
	}
}
