package swarming

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/util"

	"github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"github.com/satori/go.uuid"
	"github.com/skia-dev/glog"
)

type TestClient struct {
	botList    []*swarming.SwarmingRpcsBotInfo
	botListMtx sync.RWMutex

	taskList    []*swarming.SwarmingRpcsTaskRequestMetadata
	taskListMtx sync.RWMutex

	triggerFailure map[string]bool
	triggerMtx     sync.Mutex
}

func NewTestClient() *TestClient {
	return &TestClient{
		botList:        []*swarming.SwarmingRpcsBotInfo{},
		taskList:       []*swarming.SwarmingRpcsTaskRequestMetadata{},
		triggerFailure: map[string]bool{},
	}
}

func (c *TestClient) SwarmingService() *swarming.Service {
	return nil
}

func (c *TestClient) ListBots(dimensions map[string]string) ([]*swarming.SwarmingRpcsBotInfo, error) {
	c.botListMtx.RLock()
	defer c.botListMtx.RUnlock()
	rv := make([]*swarming.SwarmingRpcsBotInfo, 0, len(c.botList))
	for _, b := range c.botList {
		match := true
		for k, v := range dimensions {
			dMatch := false
			for _, dim := range b.Dimensions {
				if dim.Key == k && util.In(v, dim.Value) {
					dMatch = true
					break
				}
			}
			if !dMatch {
				match = false
				break
			}
		}
		if match {
			rv = append(rv, b)
		}
	}
	return rv, nil
}

func (c *TestClient) ListFreeBots(pool string) ([]*swarming.SwarmingRpcsBotInfo, error) {
	bots, err := c.ListBots(map[string]string{
		DIMENSION_POOL_KEY: pool,
	})
	if err != nil {
		return nil, err
	}
	rv := make([]*swarming.SwarmingRpcsBotInfo, 0, len(bots))
	for _, b := range bots {
		if !b.Quarantined && !b.IsDead && b.TaskId == "" {
			rv = append(rv, b)
		}
	}
	return rv, nil
}

func (c *TestClient) ListSkiaBots() ([]*swarming.SwarmingRpcsBotInfo, error) {
	return c.ListBots(map[string]string{
		DIMENSION_POOL_KEY: DIMENSION_POOL_VALUE_SKIA,
	})
}

func (c *TestClient) ListSkiaCTBots() ([]*swarming.SwarmingRpcsBotInfo, error) {
	return c.ListBots(map[string]string{
		DIMENSION_POOL_KEY: DIMENSION_POOL_VALUE_SKIA_CT,
	})
}

func (c *TestClient) ListCTBots() ([]*swarming.SwarmingRpcsBotInfo, error) {
	return c.ListBots(map[string]string{
		DIMENSION_POOL_KEY: DIMENSION_POOL_VALUE_CT,
	})
}

func (c *TestClient) GetStdoutOfTask(id string) (*swarming.SwarmingRpcsTaskOutput, error) {
	return nil, nil
}

func (c *TestClient) GracefullyShutdownBot(id string) (*swarming.SwarmingRpcsTerminateResponse, error) {
	return nil, nil
}

func (c *TestClient) ListTasks(start, end time.Time, tags []string, state string) ([]*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	c.taskListMtx.RLock()
	defer c.taskListMtx.RUnlock()
	rv := make([]*swarming.SwarmingRpcsTaskRequestMetadata, 0, len(c.taskList))
	tagSet := util.NewStringSet(tags)
	for _, t := range c.taskList {
		created, err := time.Parse(TIMESTAMP_FORMAT, t.TaskResult.CreatedTs)
		if err != nil {
			return nil, err
		}
		if !util.TimeIsZero(start) && start.After(created) {
			continue
		}
		if !util.TimeIsZero(end) && end.Before(created) {
			continue
		}
		if len(tagSet.Intersect(util.NewStringSet(t.Request.Tags))) == len(tags) {
			if state == "" || t.TaskResult.State == state {
				rv = append(rv, t)
			}
		}
	}
	return rv, nil
}

func (c *TestClient) ListSkiaTasks(start, end time.Time) ([]*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	return c.ListTasks(start, end, []string{"pool:Skia"}, "")
}

func (c *TestClient) ListTaskResults(start, end time.Time, tags []string, state string, includePerformanceStats bool) ([]*swarming.SwarmingRpcsTaskResult, error) {
	tasks, err := c.ListTasks(start, end, tags, state)
	if err != nil {
		return nil, err
	}
	rv := make([]*swarming.SwarmingRpcsTaskResult, len(tasks), len(tasks))
	for i, t := range tasks {
		rv[i] = t.TaskResult
	}
	return rv, nil
}

func (c *TestClient) CancelTask(id string) error {
	return nil
}

// md5Tags returns a MD5 hash of the task tags, excluding task ID.
func md5Tags(tags []string) string {
	filtered := make([]string, 0, len(tags))
	for _, t := range tags {
		if !strings.HasPrefix(t, "sk_id") {
			filtered = append(filtered, t)
		}
	}
	sort.Strings(filtered)
	rv, err := util.MD5SSlice(filtered)
	if err != nil {
		glog.Fatal(err)
	}
	return rv
}

// TriggerTask automatically appends its result to the mocked tasks set by
// MockTasks.
func (c *TestClient) TriggerTask(t *swarming.SwarmingRpcsNewTaskRequest) (*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	c.triggerMtx.Lock()
	defer c.triggerMtx.Unlock()
	md5 := md5Tags(t.Tags)
	if c.triggerFailure[md5] {
		delete(c.triggerFailure, md5)
		return nil, fmt.Errorf("Mocked trigger failure!")
	}

	createdTs := time.Now().UTC().Format(TIMESTAMP_FORMAT)
	id := uuid.NewV5(uuid.NewV1(), uuid.NewV4().String()).String()
	rv := &swarming.SwarmingRpcsTaskRequestMetadata{
		Request: &swarming.SwarmingRpcsTaskRequest{
			CreatedTs:      createdTs,
			ExpirationSecs: t.ExpirationSecs,
			Name:           t.Name,
			Priority:       t.Priority,
			Properties:     t.Properties,
			Tags:           t.Tags,
		},
		TaskId: id,
		TaskResult: &swarming.SwarmingRpcsTaskResult{
			CreatedTs: createdTs,
			Name:      t.Name,
			State:     "PENDING",
			TaskId:    id,
			Tags:      t.Tags,
		},
	}
	c.taskListMtx.Lock()
	defer c.taskListMtx.Unlock()
	c.taskList = append(c.taskList, rv)
	return rv, nil
}

func (c *TestClient) RetryTask(t *swarming.SwarmingRpcsTaskRequestMetadata) (*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	return c.TriggerTask(&swarming.SwarmingRpcsNewTaskRequest{
		Name:     t.Request.Name,
		Priority: t.Request.Priority,
		Tags:     t.Request.Tags,
		User:     t.Request.User,
	})
}

func (c *TestClient) GetTask(id string, includePerformanceStats bool) (*swarming.SwarmingRpcsTaskResult, error) {
	m, err := c.GetTaskMetadata(id)
	if err != nil {
		return nil, err
	}
	return m.TaskResult, nil
}

func (c *TestClient) GetTaskMetadata(id string) (*swarming.SwarmingRpcsTaskRequestMetadata, error) {
	c.taskListMtx.RLock()
	defer c.taskListMtx.RUnlock()
	for _, t := range c.taskList {
		if t.TaskId == id {
			return t, nil
		}
	}
	return nil, fmt.Errorf("No such task: %s", id)
}

func (c *TestClient) MockBots(bots []*swarming.SwarmingRpcsBotInfo) {
	c.botListMtx.Lock()
	defer c.botListMtx.Unlock()
	c.botList = bots
}

// MockTasks sets the tasks that can be returned from ListTasks, ListSkiaTasks,
// GetTaskMetadata, and GetTask. Replaces any previous tasks, including those
// automatically added by TriggerTask.
func (c *TestClient) MockTasks(tasks []*swarming.SwarmingRpcsTaskRequestMetadata) {
	c.taskListMtx.Lock()
	defer c.taskListMtx.Unlock()
	c.taskList = tasks
}

// DoMockTasks calls f for each mocked task, allowing goroutine-safe updates. f
// must not call any other method on c.
func (c *TestClient) DoMockTasks(f func(*swarming.SwarmingRpcsTaskRequestMetadata)) {
	c.taskListMtx.Lock()
	defer c.taskListMtx.Unlock()
	for _, task := range c.taskList {
		f(task)
	}
}

// MockTriggerTaskFailure forces the next call to TriggerTask which matches
// the given tags to fail.
func (c *TestClient) MockTriggerTaskFailure(tags []string) {
	c.triggerMtx.Lock()
	defer c.triggerMtx.Unlock()
	c.triggerFailure[md5Tags(tags)] = true
}
