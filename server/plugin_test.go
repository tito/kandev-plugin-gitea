package main

import (
	"context"
	"testing"
	"time"

	"github.com/kandev/kandev/pkg/pluginsdk"
	"github.com/stretchr/testify/require"
)

func TestNewPluginSetsProviderIdentity(t *testing.T) {
	p := newPlugin()
	require.Equal(t, "gitea", p.Extension.ProviderID)
	require.Equal(t, "gitea_pull_requests", p.Extension.ReferenceSource)
}

func TestOnEventDoesNotPanic(t *testing.T) {
	p := newPlugin()
	err := p.OnEvent(context.Background(), &pluginsdk.Event{EventID: "e1", EventType: "task.created"})
	require.NoError(t, err)
}

func TestConnectionGetReturnsDisconnected(t *testing.T) {
	p := newPlugin()
	resp, err := p.HandleAction(context.Background(), &pluginsdk.PluginActionRequest{
		ActionKey: "connection.get",
		Context:   pluginsdk.VerifiedActionContext{WorkspaceID: "workspace-1"},
	})
	require.NoError(t, err)
	require.Contains(t, string(resp.Body), `"connected":false`)
}

func TestUnknownActionDelegatesToExtension(t *testing.T) {
	p := newPlugin()
	_, err := p.HandleAction(context.Background(), &pluginsdk.PluginActionRequest{
		ActionKey: "repositories.list",
		Context:   pluginsdk.VerifiedActionContext{WorkspaceID: "workspace-1"},
		Body:      []byte(`{"query":"test"}`),
	})
	require.Error(t, err, "extension has no repository lister configured")
}

func gitStatusEvent(sessionID, taskID string, remoteAhead int, remoteBranch string) *pluginsdk.Event {
	return &pluginsdk.Event{
		EventID:   "e-" + sessionID,
		EventType: "git.event." + sessionID,
		Payload: map[string]any{
			"type":       "status_update",
			"session_id": sessionID,
			"task_id":    taskID,
			"status": map[string]any{
				"branch":          "feature/x",
				"remote_branch":   remoteBranch,
				"remote_ahead":    float64(remoteAhead),
				"repository_name": "backend",
			},
		},
	}
}

func TestOnEventLinksAfterDetectedPush(t *testing.T) {
	p := newPlugin()
	type call struct{ taskID, repositoryName, branch string }
	calls := make(chan call, 4)
	p.linkAfterPush = func(_ context.Context, taskID, repositoryName, branch string) error {
		calls <- call{taskID, repositoryName, branch}
		return nil
	}

	require.NoError(t, p.OnEvent(context.Background(), gitStatusEvent("s1", "t1", 2, "origin/feature/x")))
	require.NoError(t, p.OnEvent(context.Background(), gitStatusEvent("s1", "t1", 0, "origin/feature/x")))

	select {
	case c := <-calls:
		require.Equal(t, call{"t1", "backend", "feature/x"}, c)
	case <-time.After(2 * time.Second):
		t.Fatal("expected push link call")
	}

	require.NoError(t, p.OnEvent(context.Background(), gitStatusEvent("s1", "t1", 0, "origin/feature/x")))
	select {
	case <-calls:
		t.Fatal("unexpected second link call for an already synced branch")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestOnEventIgnoresNonStatusGitEvents(t *testing.T) {
	p := newPlugin()
	called := false
	p.linkAfterPush = func(context.Context, string, string, string) error { called = true; return nil }
	require.NoError(t, p.OnEvent(context.Background(), &pluginsdk.Event{
		EventType: "git.event.s1",
		Payload:   map[string]any{"type": "commit_created", "session_id": "s1", "task_id": "t1"},
	}))
	require.False(t, called)
}
