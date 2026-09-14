package plugin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseGiteaRepoURL_FullURL(t *testing.T) {
	owner, name := parseGiteaRepoURL("https://gitea.example.com", "https://gitea.example.com/myorg/myrepo")
	require.Equal(t, "myorg", owner)
	require.Equal(t, "myrepo", name)
}

func TestParseGiteaRepoURL_WithGitSuffix(t *testing.T) {
	owner, name := parseGiteaRepoURL("https://gitea.example.com", "https://gitea.example.com/myorg/myrepo.git")
	require.Equal(t, "myorg", owner)
	require.Equal(t, "myrepo", name)
}

func TestParseGiteaRepoURL_WithPathPrefix(t *testing.T) {
	owner, name := parseGiteaRepoURL("https://example.com/gitea/", "https://example.com/gitea/myorg/myrepo")
	require.Equal(t, "myorg", owner)
	require.Equal(t, "myrepo", name)
}

func TestParseGiteaRepoURL_DifferentHost(t *testing.T) {
	owner, name := parseGiteaRepoURL("https://gitea.example.com", "https://github.com/myorg/myrepo")
	require.Empty(t, owner)
	require.Empty(t, name)
}

func TestParseGiteaRepoURL_Empty(t *testing.T) {
	owner, name := parseGiteaRepoURL("https://gitea.example.com", "")
	require.Empty(t, owner)
	require.Empty(t, name)
}

func TestParseGiteaRepoURL_IncompletePath(t *testing.T) {
	owner, name := parseGiteaRepoURL("https://gitea.example.com", "https://gitea.example.com/onlyone")
	require.Empty(t, owner)
	require.Empty(t, name)
}

func TestParseGiteaRepoURL_SubpathLikeIssues(t *testing.T) {
	owner, name := parseGiteaRepoURL("https://gitea.example.com", "https://gitea.example.com/myorg/myrepo/issues/42")
	require.Equal(t, "myorg", owner)
	require.Equal(t, "myrepo", name)
}

func TestConnectionScope_Format(t *testing.T) {
	scope := ConnectionScope("https://gitea.example.com", "admin")
	require.Equal(t, "https://gitea.example.com|admin", scope)
}
