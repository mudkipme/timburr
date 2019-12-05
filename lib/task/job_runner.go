package task

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/mudkipme/timburr/utils"
	log "github.com/sirupsen/logrus"
)

type JobRunnerExecutor struct {
	endpoint      string
	excludeFields []string
	client        *http.Client
}

func DefaultJobRunnerExecutor() *JobRunnerExecutor {
	return NewJobRunnerExecutor(utils.Config.JobRunner.Endpoint, utils.Config.JobRunner.ExcludeFields)
}

func NewJobRunnerExecutor(endpoint string, excludeFields []string) *JobRunnerExecutor {
	return &JobRunnerExecutor{
		endpoint:      endpoint,
		excludeFields: excludeFields,
		client: &http.Client{
			Timeout: time.Second * 180,
		},
	}
}

func (t *JobRunnerExecutor) Execute(message []byte) error {
	var messageMap map[string]interface{}
	if err := json.Unmarshal(message, &messageMap); err != nil {
		return err
	}
	for _, f := range t.excludeFields {
		delete(messageMap, f)
	}
	rb, err := json.Marshal(messageMap)
	if err != nil {
		return err
	}
	err = t.doExecute(rb)
	if err != nil {
		return err
	}
	log.WithFields(log.Fields(messageMap)).Info("job executed")
	return nil
}

func (t *JobRunnerExecutor) doExecute(message []byte) error {
	resp, err := http.Post(t.endpoint, "application/json", bytes.NewBuffer(message))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("job runner execute failed, response: %v", string(body))
	}
	return nil
}
