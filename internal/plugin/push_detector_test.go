package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPushDetectorFirstObservationSynced(t *testing.T) {
	d := NewPushDetector()
	require.True(t, d.Observe("s1", "", 0, "origin/feature"))
}

func TestPushDetectorNoRemoteBranch(t *testing.T) {
	d := NewPushDetector()
	require.False(t, d.Observe("s1", "", 0, ""))
	require.False(t, d.Observe("s1", "", 2, ""))
}

func TestPushDetectorUnsyncedToSynced(t *testing.T) {
	d := NewPushDetector()
	require.False(t, d.Observe("s1", "", 0, ""))
	require.True(t, d.Observe("s1", "", 0, "origin/feature"))
}

func TestPushDetectorAheadToSynced(t *testing.T) {
	d := NewPushDetector()
	require.True(t, d.Observe("s1", "", 0, "origin/feature"))
	require.False(t, d.Observe("s1", "", 3, "origin/feature"))
	require.True(t, d.Observe("s1", "", 0, "origin/feature"))
}

func TestPushDetectorAlreadySynced(t *testing.T) {
	d := NewPushDetector()
	require.True(t, d.Observe("s1", "", 0, "origin/feature"))
	require.False(t, d.Observe("s1", "", 0, "origin/feature"))
}

func TestPushDetectorKeysPerSessionAndRepository(t *testing.T) {
	d := NewPushDetector()
	require.True(t, d.Observe("s1", "backend", 0, "origin/feature"))
	require.True(t, d.Observe("s1", "web", 0, "origin/feature"))
	require.True(t, d.Observe("s2", "backend", 0, "origin/feature"))
	require.False(t, d.Observe("s1", "backend", 0, "origin/feature"))
}

func TestParseGitStatusEvent(t *testing.T) {
	ev, ok := ParseGitStatusEvent(map[string]any{
		"type":       "status_update",
		"session_id": "s1",
		"task_id":    "t1",
		"status": map[string]any{
			"branch":          "feature",
			"remote_branch":   "origin/feature",
			"remote_ahead":    float64(2),
			"repository_name": "backend",
		},
	})
	require.True(t, ok)
	require.Equal(t, GitStatusEvent{
		SessionID: "s1", TaskID: "t1", Branch: "feature",
		RemoteBranch: "origin/feature", RemoteAhead: 2, RepositoryName: "backend",
	}, ev)
}

func TestParseGitStatusEventRejectsOtherTypes(t *testing.T) {
	_, ok := ParseGitStatusEvent(map[string]any{"type": "commit_created", "session_id": "s1", "task_id": "t1", "status": map[string]any{}})
	require.False(t, ok)
	_, ok = ParseGitStatusEvent(map[string]any{"type": "status_update", "session_id": "s1", "task_id": "t1"})
	require.False(t, ok)
	_, ok = ParseGitStatusEvent(map[string]any{"type": "status_update", "session_id": "s1", "status": map[string]any{}})
	require.False(t, ok)
	_, ok = ParseGitStatusEvent(nil)
	require.False(t, ok)
}
