package plugin

import (
	"context"
	"testing"

	"github.com/kandev/kandev/pkg/pluginsdk"
	"github.com/stretchr/testify/require"
)

func TestMatchCredentialOrigin_MatchesHost(t *testing.T) {
	err := matchCredentialOrigin("https://gitea.example.com", "gitea.example.com", "org/repo.git")
	require.NoError(t, err)
}

func TestMatchCredentialOrigin_RejectsMismatchedHost(t *testing.T) {
	err := matchCredentialOrigin("https://gitea.example.com", "other.example.com", "org/repo.git")
	require.ErrorContains(t, err, "does not match")
}

func TestMatchCredentialOrigin_MatchesPathPrefix(t *testing.T) {
	err := matchCredentialOrigin("https://example.com/gitea", "example.com", "gitea/org/repo.git")
	require.NoError(t, err)
}

func TestMatchCredentialOrigin_RejectsMismatchedPath(t *testing.T) {
	err := matchCredentialOrigin("https://example.com/gitea", "example.com", "other/org/repo.git")
	require.ErrorContains(t, err, "does not match")
}

func TestValidateCredentialScope_RejectsEmptyFields(t *testing.T) {
	host := newFakeHost()
	err := validateCredentialScope(context.Background(), host, &pluginsdk.ResolveGitCredentialRequest{})
	require.ErrorContains(t, err, "requires")
}

func TestValidateCredentialScope_RejectsNonGiteaRepository(t *testing.T) {
	host := newFakeHost()
	host.repos = []pluginsdk.Repository{
		{ID: "repo-1", WorkspaceID: "ws-1", ProviderID: "github"},
	}
	err := validateCredentialScope(context.Background(), host, &pluginsdk.ResolveGitCredentialRequest{
		WorkspaceID:  "ws-1",
		TaskID:       "task-1",
		RepositoryID: "repo-1",
	})
	require.ErrorContains(t, err, "not a gitea-provider")
}

func TestValidateCredentialScope_AcceptsGiteaRepository(t *testing.T) {
	host := newFakeHost()
	host.repos = []pluginsdk.Repository{
		{ID: "repo-1", WorkspaceID: "ws-1", ProviderID: "gitea"},
	}
	err := validateCredentialScope(context.Background(), host, &pluginsdk.ResolveGitCredentialRequest{
		WorkspaceID:  "ws-1",
		TaskID:       "task-1",
		RepositoryID: "repo-1",
	})
	require.NoError(t, err)
}

func TestGetGitCredentialBinding(t *testing.T) {
	host := newFakeHost()
	host.setState("workspace", "ws-1", connectionStateKey, map[string]any{
		"base_url": "https://gitea.example.com", "login": "admin", "generation": 3,
	})
	resp, err := GetGitCredentialBinding(context.Background(), host, &pluginsdk.GitCredentialBindingRequest{
		WorkspaceID: "ws-1",
	})
	require.NoError(t, err)
	require.Equal(t, "gitea-credential:3", resp.Binding)
}
