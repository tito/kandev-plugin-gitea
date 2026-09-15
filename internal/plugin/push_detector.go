package plugin

import "sync"

const pushUnsynced = -1

type PushDetector struct {
	mu      sync.Mutex
	tracker map[string]int
}

func NewPushDetector() *PushDetector {
	return &PushDetector{tracker: make(map[string]int)}
}

// Observe records one status observation and reports whether it completed a push.
func (d *PushDetector) Observe(sessionID, repositoryName string, remoteAhead int, remoteBranch string) bool {
	key := sessionID + "|" + repositoryName
	value := remoteAhead
	if remoteBranch == "" {
		value = pushUnsynced
	}
	d.mu.Lock()
	prev, loaded := d.tracker[key]
	d.tracker[key] = value
	d.mu.Unlock()
	if remoteBranch == "" || remoteAhead != 0 {
		return false
	}
	if !loaded {
		return true
	}
	return prev != 0
}

type GitStatusEvent struct {
	SessionID      string
	TaskID         string
	Branch         string
	RemoteBranch   string
	RemoteAhead    int
	RepositoryName string
}

// ParseGitStatusEvent extracts a status_update git event; ok is false for any other payload.
func ParseGitStatusEvent(payload map[string]any) (GitStatusEvent, bool) {
	if payload == nil {
		return GitStatusEvent{}, false
	}
	if kind, _ := payload["type"].(string); kind != "status_update" {
		return GitStatusEvent{}, false
	}
	status, _ := payload["status"].(map[string]any)
	if status == nil {
		return GitStatusEvent{}, false
	}
	ev := GitStatusEvent{
		SessionID:      stringField(payload, "session_id"),
		TaskID:         stringField(payload, "task_id"),
		Branch:         stringField(status, "branch"),
		RemoteBranch:   stringField(status, "remote_branch"),
		RemoteAhead:    intField(status, "remote_ahead"),
		RepositoryName: stringField(status, "repository_name"),
	}
	if ev.SessionID == "" || ev.TaskID == "" {
		return GitStatusEvent{}, false
	}
	return ev, true
}

func stringField(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func intField(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	default:
		return 0
	}
}
