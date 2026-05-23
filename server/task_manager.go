package server

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"reg_go/internal/core"
	"reg_go/internal/email"
)

type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskStopped   TaskStatus = "stopped"
)

type TaskConfig struct {
	Count       int    `json:"count"`
	Concurrency int    `json:"concurrency"`
	Delay       int    `json:"delay"`
	Proxy       string `json:"proxy"`
	UseOutlook  bool   `json:"useOutlook"`
	OutlookCSV  string `json:"outlookCsv"`
	MoEmailURL  string `json:"moEmailUrl"`
	MoEmailKey  string `json:"moEmailKey"`
}

type TaskResult struct {
	Email        string  `json:"email"`
	Status       string  `json:"status"`
	Error        string  `json:"error,omitempty"`
	Subscription string  `json:"subscription,omitempty"`
	CreditUsed   float64 `json:"creditUsed,omitempty"`
	CreditLimit  float64 `json:"creditLimit,omitempty"`
}

type Task struct {
	ID        string       `json:"id"`
	Status    TaskStatus   `json:"status"`
	Config    TaskConfig   `json:"config"`
	Results   []TaskResult `json:"results"`
	Success   int          `json:"success"`
	Failed    int          `json:"failed"`
	Total     int          `json:"total"`
	CreatedAt string       `json:"createdAt"`
	StartedAt string       `json:"startedAt,omitempty"`
	EndedAt   string       `json:"endedAt,omitempty"`

	stopCh chan struct{}
	mu     sync.Mutex
}

type TaskManager struct {
	tasks   map[string]*Task
	mu      sync.RWMutex
	logHub  *LogHub
	dataDir string
}

func NewTaskManager(logHub *LogHub, dataDir string) *TaskManager {
	return &TaskManager{
		tasks:   make(map[string]*Task),
		logHub:  logHub,
		dataDir: dataDir,
	}
}

func (tm *TaskManager) CreateTask(cfg TaskConfig) *Task {
	id := fmt.Sprintf("task-%d", time.Now().UnixMilli())
	task := &Task{
		ID:        id,
		Status:    TaskPending,
		Config:    cfg,
		Total:     cfg.Count,
		CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
		stopCh:    make(chan struct{}),
	}
	tm.mu.Lock()
	tm.tasks[id] = task
	tm.mu.Unlock()
	go tm.runTask(task)
	return task
}

func (tm *TaskManager) GetTask(id string) *Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.tasks[id]
}

func (tm *TaskManager) ListTasks() []*Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	list := make([]*Task, 0, len(tm.tasks))
	for _, t := range tm.tasks {
		list = append(list, t)
	}
	return list
}

func (tm *TaskManager) StopTask(id string) bool {
	tm.mu.RLock()
	task := tm.tasks[id]
	tm.mu.RUnlock()
	if task == nil || task.Status != TaskRunning {
		return false
	}
	close(task.stopCh)
	task.mu.Lock()
	task.Status = TaskStopped
	task.EndedAt = time.Now().Format("2006-01-02 15:04:05")
	task.mu.Unlock()
	return true
}

func (tm *TaskManager) runTask(task *Task) {
	task.mu.Lock()
	task.Status = TaskRunning
	task.StartedAt = time.Now().Format("2006-01-02 15:04:05")
	task.mu.Unlock()

	sendLog := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		log.Printf("[%s] %s", task.ID, msg)
		tm.logHub.Send(task.ID, msg)
	}

	cfg := core.NewConfig()
	cfg.Proxy = task.Config.Proxy
	cfg.MoEmailBaseURL = task.Config.MoEmailURL
	cfg.MoEmailAPIKey = task.Config.MoEmailKey

	var outlookAccounts []email.OutlookAccount
	if task.Config.UseOutlook {
		cfg.UseOutlook = true
		csvPath := task.Config.OutlookCSV
		if csvPath == "" {
			csvPath = tm.dataDir + "/outlook.csv"
		}
		cfg.OutlookCSV = csvPath
		accounts, err := email.ParseOutlookCSV(csvPath)
		if err != nil {
			sendLog("Outlook CSV 读取失败: %v", err)
			task.mu.Lock()
			task.Status = TaskCompleted
			task.EndedAt = time.Now().Format("2006-01-02 15:04:05")
			task.mu.Unlock()
			return
		}
		outlookAccounts = accounts
		sendLog("Outlook 模式: 已加载 %d 个账号", len(accounts))
	} else {
		sendLog("临时邮箱模式 (MoeMail)")
	}

	var accountIdx int64
	getNext := func() (email.OutlookAccount, int, bool) {
		idx := int(atomic.AddInt64(&accountIdx, 1) - 1)
		if idx >= len(outlookAccounts) {
			return email.OutlookAccount{}, idx, false
		}
		return outlookAccounts[idx], idx, true
	}

	doOne := func(num int) {
		for {
			select {
			case <-task.stopCh:
				return
			default:
			}

			taskCfg := *cfg
			taskCfg.Password = core.GenPassword()

			var acc email.OutlookAccount
			if cfg.UseOutlook {
				var idx int
				var ok bool
				acc, idx, ok = getNext()
				if !ok {
					sendLog("[任务%d] Outlook 账号已用完", num+1)
					return
				}
				taskCfg.OutlookAccount = &acc
				sendLog("[%d/%d] 开始注册 (#%d %s)", num+1, task.Total, idx+1, acc.Email)
			} else {
				sendLog("[%d/%d] 开始注册", num+1, task.Total)
			}

			reg := core.NewRegistrar(&taskCfg)
			result := reg.Run()

			errStr, _ := result["error"].(string)
			if errStr == "邮箱已注册过，跳过" {
				emailAddr, _ := result["email"].(string)
				if emailAddr == "" && cfg.UseOutlook {
					emailAddr = acc.Email
				}
				sendLog("[%d/%d] %s 已注册，跳过", num+1, task.Total, emailAddr)
				continue
			}

			task.mu.Lock()
			tr := TaskResult{
				Email:  fmt.Sprintf("%v", result["email"]),
				Status: fmt.Sprintf("%v", result["status"]),
			}
			if result["status"] == "success" {
				task.Success++
				if v, ok := result["verify"].(map[string]interface{}); ok {
					tr.Subscription, _ = v["subscription"].(string)
					tr.CreditUsed, _ = v["credit_used"].(float64)
					tr.CreditLimit, _ = v["credit_limit"].(float64)
				}
				sendLog("[%d/%d] %s 成功", num+1, task.Total, tr.Email)
			} else {
				task.Failed++
				tr.Error = errStr
				if len(tr.Error) > 100 {
					tr.Error = tr.Error[:100]
				}
				sendLog("[%d/%d] %s 失败: %s", num+1, task.Total, tr.Email, tr.Error)
			}
			task.Results = append(task.Results, tr)
			task.mu.Unlock()
			tm.persistResults(task)
			return
		}
	}

	count := task.Config.Count
	conc := task.Config.Concurrency
	if conc < 1 {
		conc = 1
	}

	stopped := false
	if conc > 1 {
		sendLog("并发模式: %d 并发, 共 %d 个", conc, count)
		sem := make(chan struct{}, conc)
		var wg sync.WaitGroup
		for i := 0; i < count; i++ {
			select {
			case <-task.stopCh:
				stopped = true
			default:
			}
			if stopped {
				break
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(idx int) {
				defer wg.Done()
				defer func() { <-sem }()
				doOne(idx)
			}(i)
		}
		wg.Wait()
	} else {
		for i := 0; i < count; i++ {
			select {
			case <-task.stopCh:
				stopped = true
			default:
			}
			if stopped {
				break
			}
			doOne(i)
			if task.Config.Delay > 0 && i < count-1 {
				time.Sleep(time.Duration(task.Config.Delay) * time.Second)
			}
		}
	}

	task.mu.Lock()
	if task.Status == TaskRunning {
		task.Status = TaskCompleted
	}
	if task.EndedAt == "" {
		task.EndedAt = time.Now().Format("2006-01-02 15:04:05")
	}
	task.mu.Unlock()
	sendLog("任务完成! 成功: %d, 失败: %d", task.Success, task.Failed)
}

func (tm *TaskManager) persistResults(task *Task) {
	task.mu.Lock()
	results := make([]TaskResult, len(task.Results))
	copy(results, task.Results)
	task.mu.Unlock()

	path := tm.dataDir + "/results.json"
	var existing []map[string]interface{}
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &existing)
	}

	seen := make(map[string]bool)
	for _, e := range existing {
		if em, ok := e["email"].(string); ok {
			seen[em] = true
		}
	}

	for _, r := range results {
		if r.Status == "success" && !seen[r.Email] {
			existing = append(existing, map[string]interface{}{
				"email":        r.Email,
				"subscription": r.Subscription,
				"creditUsed":   r.CreditUsed,
				"creditLimit":  r.CreditLimit,
				"provider":     "BuilderId",
				"region":       "us-east-1",
			})
			seen[r.Email] = true
		}
	}

	b, _ := json.MarshalIndent(existing, "", "  ")
	os.WriteFile(path, b, 0644)
}

func (tm *TaskManager) GetAllResults() []map[string]interface{} {
	path := tm.dataDir + "/results.json"
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var results []map[string]interface{}
	json.Unmarshal(data, &results)
	return results
}
