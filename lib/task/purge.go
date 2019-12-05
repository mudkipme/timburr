package task

import "errors"

type PurgeTaskExecutor struct {
}

func DefaultPurgeTask() *PurgeTaskExecutor {
	return &PurgeTaskExecutor{}
}

func (t *PurgeTaskExecutor) Execute(message []byte) error {
	return errors.New("not implemented")
}
