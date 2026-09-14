package main

import (
	"context"
	"testing"

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
