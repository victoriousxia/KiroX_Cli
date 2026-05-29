package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"reg_go/internal/core"
	"reg_go/internal/email"
	httputil "reg_go/internal/http"
)

type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskStopped   TaskStatus = "stopped"
)

type TaskConfig struct {
	Count          int    `json:"count"`
	Concurrency    int    `json:"concurrency"`
	Delay          int    `json:"delay"`
	Proxy          string `json:"proxy"`
	UpstreamProxy  string `json:"upstreamProxy"`
	EmailMode      string `json:"emailMode"`
	OutlookCSV     string `json:"outlookCsv"`
	MoEmailURL     string `json:"moEmailUrl"`
	MoEmailKey     string `json:"moEmailKey"`
	CfEmailURL     string `json:"cfEmailUrl"`
	CfEmailAuth    string `json:"cfEmailAuth"`
}

type TaskResult struct {
	Email         string  `json:"email"`
	Password      string  `json:"password,omitempty"`
	EmailPassword string  `json:"emailPassword,omitempty"`
	Status        string  `json:"status"`
	Error         string  `json:"error,omitempty"`
	Subscription  string  `json:"subscription,omitempty"`
	CreditUsed    float64 `json:"creditUsed,omitempty"`
	CreditLimit   float64 `json:"creditLimit,omitempty"`
	ClientID      string  `json:"clientId,omitempty"`
	ClientSecret  string  `json:"clientSecret,omitempty"`
	RefreshToken  string  `json:"refreshToken,omitempty"`
}

type TaskLog struct {
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

type Task struct {
	ID        string       `json:"id"`
	Status    TaskStatus   `json:"status"`
	Config    TaskConfig   `json:"config"`
	Results   []TaskResult `json:"results"`
	Logs      []TaskLog    `json:"logs,omitempty"`
	Success   int          `json:"success"`
	Failed    int          `json:"failed"`
	Total     int          `json:"total"`
	CreatedAt string       `json:"createdAt"`
	StartedAt string       `json:"startedAt,omitempty"`
	EndedAt   string       `json:"endedAt,omitempty"`

	stopCh   chan struct{}
	stopOnce sync.Once
	mu       sync.Mutex
}

var taskCounter int64

type TaskManager struct {
	tasks    map[string]*Task
	mu       sync.RWMutex
	logHub   *LogHub
	dataDir  string
	fileMu   sync.Mutex
}

func NewTaskManager(logHub *LogHub, dataDir string) *TaskManager {
	return &TaskManager{
		tasks:   make(map[string]*Task),
		logHub:  logHub,
		dataDir: dataDir,
	}
}

func (tm *TaskManager) CreateTask(cfg TaskConfig) *Task {
	seq := atomic.AddInt64(&taskCounter, 1)
	id := fmt.Sprintf("task-%d-%d", time.Now().UnixMilli(), seq)
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
	if task == nil {
		return false
	}
	task.mu.Lock()
	if task.Status != TaskRunning {
		task.mu.Unlock()
		return false
	}
	task.Status = TaskStopped
	task.EndedAt = time.Now().Format("2006-01-02 15:04:05")
	task.mu.Unlock()
	task.stopOnce.Do(func() { close(task.stopCh) })
	return true
}

func (tm *TaskManager) runTask(task *Task) {
	task.mu.Lock()
	task.Status = TaskRunning
	task.StartedAt = time.Now().Format("2006-01-02 15:04:05")
	task.mu.Unlock()

	sendLog := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		ts := time.Now().Format("15:04:05")
		fmt.Fprintf(os.Stderr, "[%s] [%s] %s\n", ts, task.ID, msg)
		tm.logHub.Send(task.ID, msg)
		task.mu.Lock()
		task.Logs = append(task.Logs, TaskLog{Message: msg, Timestamp: ts})
		task.mu.Unlock()
	}

	cfg := core.NewConfig()

	// Resolve upstream and primary proxy
	upstreamProxy := task.Config.UpstreamProxy
	if upstreamProxy == "" {
		upstreamProxy = os.Getenv("UPSTREAM_PROXY")
	}
	primaryProxy := task.Config.Proxy
	if primaryProxy == "" {
		primaryProxy = os.Getenv("PROXY")
	}

	var stopChain func()
	defer func() {
		if stopChain != nil {
			stopChain()
		}
	}()
	if upstreamProxy != "" && primaryProxy != "" {
		// Chain: primary → upstream → target
		localAddr, stop, err := httputil.ProxyChain(primaryProxy, upstreamProxy)
		if err != nil {
			sendLog("代理链启动失败: %v", err)
			task.mu.Lock()
			task.Status = TaskCompleted
			task.EndedAt = time.Now().Format("2006-01-02 15:04:05")
			task.mu.Unlock()
			return
		}
		stopChain = stop
		cfg.Proxy = "socks5://" + localAddr
		sendLog("代理链已启动: %s → %s", primaryProxy, upstreamProxy)
	} else if upstreamProxy != "" {
		cfg.Proxy = upstreamProxy
	} else {
		cfg.Proxy = primaryProxy
	}
	cfg.MoEmailBaseURL = task.Config.MoEmailURL
	if cfg.MoEmailBaseURL == "" {
		cfg.MoEmailBaseURL = os.Getenv("MOEMAIL_BASE_URL")
	}
	cfg.MoEmailAPIKey = task.Config.MoEmailKey
	if cfg.MoEmailAPIKey == "" {
		cfg.MoEmailAPIKey = os.Getenv("MOEMAIL_API_KEY")
	}
	cfg.CfEmailBaseURL = task.Config.CfEmailURL
	if cfg.CfEmailBaseURL == "" {
		cfg.CfEmailBaseURL = os.Getenv("CF_EMAIL_BASE_URL")
	}
	cfg.CfEmailAuth = task.Config.CfEmailAuth
	if cfg.CfEmailAuth == "" {
		cfg.CfEmailAuth = os.Getenv("CF_EMAIL_AUTH")
	}

	emailMode := task.Config.EmailMode
	if emailMode == "" {
		emailMode = "moemail"
	}
	cfg.EmailMode = emailMode

	var outlookAccounts []email.OutlookAccount
	var csvPath string
	if emailMode == "outlook" {
		csvPath = task.Config.OutlookCSV
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
	} else if emailMode == "cloudflare" {
		sendLog("临时邮箱模式 (Cloudflare Temp Email)")
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
			if emailMode == "outlook" {
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
				if emailAddr == "" && emailMode == "outlook" {
					emailAddr = acc.Email
				}
				sendLog("[%d/%d] %s 已注册，跳过", num+1, task.Total, emailAddr)
				if emailMode == "outlook" && csvPath != "" && emailAddr != "" {
					tm.removeAccountFromCSV(csvPath, emailAddr)
					sendLog("[%d/%d] %s 已从账号池移除 (已注册)", num+1, task.Total, emailAddr)
				}
				continue
			}

			task.mu.Lock()
			tr := TaskResult{
				Status: fmt.Sprintf("%v", result["status"]),
			}
			tr.Email, _ = result["email"].(string)

			statusVal, _ := result["status"].(string)
			if statusVal == "success" {
				task.Success++
				tr.Password, _ = result["password"].(string)
				if emailMode == "outlook" && taskCfg.OutlookAccount != nil {
					tr.EmailPassword = taskCfg.OutlookAccount.Password
				}
				tr.ClientID, _ = result["client_id"].(string)
				tr.ClientSecret, _ = result["client_secret"].(string)
				if at, ok := result["aws_token"].(map[string]interface{}); ok {
					tr.RefreshToken, _ = at["refreshToken"].(string)
				}
				if v, ok := result["verify"].(map[string]interface{}); ok {
					tr.Subscription, _ = v["subscription"].(string)
					tr.CreditUsed, _ = v["credit_used"].(float64)
					tr.CreditLimit, _ = v["credit_limit"].(float64)
				}
			} else {
				task.Failed++
				tr.Error = errStr
			}
			task.Results = append(task.Results, tr)
			task.mu.Unlock()

			if statusVal == "success" {
				sendLog("[%d/%d] %s 注册成功!", num+1, task.Total, tr.Email)
				if emailMode == "outlook" && csvPath != "" && tr.Email != "" {
					tm.removeAccountFromCSV(csvPath, tr.Email)
					sendLog("[%d/%d] %s 已从账号池移除 (注册成功)", num+1, task.Total, tr.Email)
				}
			} else {
				sendLog("[%d/%d] %s 失败: %s (status=%v)", num+1, task.Total, tr.Email, tr.Error, result["status"])
				if emailMode == "outlook" && csvPath != "" && tr.Email != "" {
					if !isTransientError(tr.Error) {
						tm.removeAccountFromCSV(csvPath, tr.Email)
						sendLog("[%d/%d] %s 已从账号池移除 (不可恢复错误)", num+1, task.Total, tr.Email)
					} else {
						sendLog("[%d/%d] %s 保留在账号池 (网络临时错误，可重试)", num+1, task.Total, tr.Email)
					}
				}
			}
			tm.persistResults(task)
			return
		}
	}

	count := task.Config.Count
	conc := task.Config.Concurrency
	if conc < 1 {
		conc = 1
	}

	// Capture global log output for the entire task execution
	logReader, logWriter := io.Pipe()
	origOutput := log.Writer()
	log.SetOutput(io.MultiWriter(origOutput, logWriter))
	logDone := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := logReader.Read(buf)
			if n > 0 {
				lines := strings.Split(strings.TrimSpace(string(buf[:n])), "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line != "" {
						ts := time.Now().Format("15:04:05")
						tm.logHub.Send(task.ID, line)
						task.mu.Lock()
						task.Logs = append(task.Logs, TaskLog{Message: line, Timestamp: ts})
						task.mu.Unlock()
					}
				}
			}
			if err != nil {
				break
			}
		}
		close(logDone)
	}()

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

	// Stop capturing global log output
	log.SetOutput(origOutput)
	logWriter.Close()
	<-logDone

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

	tm.fileMu.Lock()
	defer tm.fileMu.Unlock()

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
			entry := map[string]interface{}{
				"email":        r.Email,
				"password":     r.Password,
				"refreshToken": r.RefreshToken,
				"clientId":     r.ClientID,
				"clientSecret": r.ClientSecret,
				"subscription": r.Subscription,
				"creditUsed":   r.CreditUsed,
				"creditLimit":  r.CreditLimit,
				"provider":     "BuilderId",
				"region":       "us-east-1",
			}
			if r.EmailPassword != "" {
				entry["emailPassword"] = r.EmailPassword
			}
			existing = append(existing, entry)
			seen[r.Email] = true
		}
	}

	b, _ := json.MarshalIndent(existing, "", "  ")
	os.WriteFile(path, b, 0644)
}

func (tm *TaskManager) GetAllResults() []map[string]interface{} {
	tm.fileMu.Lock()
	defer tm.fileMu.Unlock()

	path := tm.dataDir + "/results.json"
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var results []map[string]interface{}
	json.Unmarshal(data, &results)

	csvPath := tm.dataDir + "/outlook.csv"
	if accounts, err := email.ParseOutlookCSV(csvPath); err == nil && len(accounts) > 0 {
		lookup := make(map[string]string, len(accounts))
		for _, acc := range accounts {
			lookup[acc.Email] = acc.Password
		}
		for i, r := range results {
			if _, has := r["emailPassword"]; !has {
				if em, ok := r["email"].(string); ok {
					if pass, found := lookup[em]; found {
						results[i]["emailPassword"] = pass
					}
				}
			}
		}
	}

	return results
}

func isTransientError(errStr string) bool {
	transient := []string{
		"EOF", "timeout", "context deadline",
		"connection refused", "dial tcp", "TLS handshake",
		"connection reset", "no such host",
		"等待验证码超时",
	}
	lower := strings.ToLower(errStr)
	for _, t := range transient {
		if strings.Contains(lower, strings.ToLower(t)) {
			return true
		}
	}
	return false
}

func (tm *TaskManager) removeAccountFromCSV(csvPath, emailAddr string) {
	tm.fileMu.Lock()
	defer tm.fileMu.Unlock()

	data, err := os.ReadFile(csvPath)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	var newLines []string
	for _, line := range lines {
		trimLine := strings.TrimSpace(line)
		if trimLine == "" {
			continue
		}
		if !strings.HasPrefix(trimLine, emailAddr+"----") {
			newLines = append(newLines, line)
		}
	}
	content := strings.Join(newLines, "\n") + "\n"
	if err := os.WriteFile(csvPath, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "[removeCSV] 写入失败: %v\n", err)
	}
}
