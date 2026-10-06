package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Gordonynh/ctfile-down/internal/ctfile"
)

// Status is a download task's lifecycle state.
type Status string

const (
	StatusResolving   Status = "resolving"
	StatusDownloading Status = "downloading"
	StatusDone        Status = "done"
	StatusError       Status = "error"
	StatusCanceled    Status = "canceled"
)

// Task is the JSON-facing snapshot of a download job.
type Task struct {
	ID         string  `json:"id"`
	URL        string  `json:"url"`
	FileName   string  `json:"file_name"`
	Total      int64   `json:"total"`
	Downloaded int64   `json:"downloaded"`
	Progress   float64 `json:"progress"`
	Speed      int64   `json:"speed"`
	Status     Status  `json:"status"`
	Error      string  `json:"error,omitempty"`
	SavePath   string  `json:"save_path"`
	CreatedAt  int64   `json:"created_at"`
}

// job holds the mutable runtime state of a download task. It is separate from
// Task so that the public snapshot value carries no locks.
type job struct {
	Task      Task
	cancel    context.CancelFunc
	mu        sync.Mutex
	lastBytes int64
	lastTime  time.Time
}

func (j *job) snapshot() Task {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.Task
}

func (j *job) setStatus(s Status) {
	j.mu.Lock()
	j.Task.Status = s
	j.mu.Unlock()
}

func (j *job) setError(msg string) {
	j.mu.Lock()
	j.Task.Error = msg
	j.mu.Unlock()
}

// Manager tracks download tasks in memory.
type Manager struct {
	mu      sync.RWMutex
	jobs    map[string]*job
	order   []string
	saveDir string
}

// NewManager creates a task manager writing to saveDir.
func NewManager(saveDir string) *Manager {
	if saveDir == "" {
		saveDir = "./downloads"
	}
	return &Manager{jobs: map[string]*job{}, saveDir: saveDir}
}

// SaveDir returns the directory downloads are written to.
func (m *Manager) SaveDir() string {
	return m.saveDir
}

// Start resolves a link and begins downloading it in the background.
func (m *Manager) Start(rawURL, passcode string) (Task, error) {
	link, err := ctfile.ParseLink(rawURL)
	if err != nil {
		return Task{}, err
	}
	if passcode != "" {
		link.Passcode = passcode
	}
	client := ctfile.New(link)
	info, err := client.Resolve()
	if err != nil {
		return Task{}, err
	}
	t, err := client.GetDownloadURL(info)
	if err != nil {
		return Task{}, err
	}

	id := newID()
	j := &job{
		Task: Task{
			ID:        id,
			URL:       rawURL,
			FileName:  info.FileName,
			Total:     t.FileSize,
			Status:    StatusResolving,
			SavePath:  filepath.Join(m.saveDir, info.FileName),
			CreatedAt: time.Now().Unix(),
		},
		lastTime: time.Now(),
	}
	ctx, cancel := context.WithCancel(context.Background())
	j.cancel = cancel

	m.mu.Lock()
	m.jobs[id] = j
	m.order = append(m.order, id)
	m.mu.Unlock()

	go m.run(ctx, j, client, info)
	return j.snapshot(), nil
}

func (m *Manager) run(ctx context.Context, j *job, client *ctfile.Client, info *ctfile.FileInfo) {
	j.setStatus(StatusDownloading)
	err := client.Download(ctx, info, j.Task.SavePath, func(done, total int64) {
		j.mu.Lock()
		j.Task.Downloaded = done
		j.Task.Total = total
		if total > 0 {
			j.Task.Progress = 100 * float64(done) / float64(total)
		}
		now := time.Now()
		if elapsed := now.Sub(j.lastTime).Seconds(); elapsed >= 1 {
			j.Task.Speed = int64(float64(done-j.lastBytes) / elapsed)
			j.lastBytes = done
			j.lastTime = now
		}
		j.mu.Unlock()
	})
	if err != nil {
		if ctx.Err() != nil {
			j.setStatus(StatusCanceled)
		} else {
			j.setStatus(StatusError)
			j.setError(err.Error())
		}
		return
	}
	j.mu.Lock()
	j.Task.Downloaded = j.Task.Total
	j.Task.Progress = 100
	j.Task.Speed = 0
	j.mu.Unlock()
	j.setStatus(StatusDone)
}

// List returns all tasks, newest first.
func (m *Manager) List() []Task {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Task, 0, len(m.order))
	for i := len(m.order) - 1; i >= 0; i-- {
		if j := m.jobs[m.order[i]]; j != nil {
			out = append(out, j.snapshot())
		}
	}
	return out
}

// Get returns a single task by id.
func (m *Manager) Get(id string) *Task {
	m.mu.RLock()
	j := m.jobs[id]
	m.mu.RUnlock()
	if j == nil {
		return nil
	}
	s := j.snapshot()
	return &s
}

// Cancel stops a running task.
func (m *Manager) Cancel(id string) {
	m.mu.RLock()
	j := m.jobs[id]
	m.mu.RUnlock()
	if j != nil && j.cancel != nil {
		j.cancel()
	}
}

// Remove cancels and forgets a task, and deletes its partial files.
func (m *Manager) Remove(id string) {
	m.mu.RLock()
	j := m.jobs[id]
	m.mu.RUnlock()
	if j == nil {
		return
	}
	if j.cancel != nil {
		j.cancel()
	}
	m.mu.Lock()
	delete(m.jobs, id)
	for i, v := range m.order {
		if v == id {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
	m.mu.Unlock()
	os.RemoveAll(j.Task.SavePath + ".parts")
}

func newID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
