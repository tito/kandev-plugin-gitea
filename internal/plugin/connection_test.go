package plugin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConnectionScope(t *testing.T) {
	require.Equal(t, "https://gitea.example.com|admin", ConnectionScope("https://gitea.example.com/", "admin"))
	require.Equal(t, "https://gitea.example.com|admin", ConnectionScope("https://gitea.example.com", "admin"))
}

func TestSaveConnectionRequiresBaseURLAndPAT(t *testing.T) {
	host := newFakeHost()
	_, err := SaveConnection(context.Background(), host, "ws-1", "", "token")
	require.ErrorContains(t, err, "required")

	_, err = SaveConnection(context.Background(), host, "ws-1", "https://gitea.example.com", "")
	require.ErrorContains(t, err, "required")
}

func TestGetConnectionReturnsNilWhenNotConfigured(t *testing.T) {
	host := newFakeHost()
	conn, err := GetConnection(context.Background(), host, "ws-1")
	require.NoError(t, err)
	require.Nil(t, conn)
}

func TestDisconnectDeletesStateAndSecret(t *testing.T) {
	host := newFakeHost()
	host.setState("workspace", "ws-1", connectionStateKey, map[string]any{
		"base_url": "https://gitea.example.com", "login": "admin", "generation": 1,
	})
	host.setSecret(patSecretPrefix+"ws-1", "token-value")

	err := Disconnect(context.Background(), host, "ws-1")
	require.NoError(t, err)

	conn, err := GetConnection(context.Background(), host, "ws-1")
	require.NoError(t, err)
	require.Nil(t, conn)

	_, found, _ := host.GetSecret(context.Background(), patSecretPrefix+"ws-1")
	require.False(t, found)
}
