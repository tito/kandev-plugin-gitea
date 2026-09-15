package plugin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kandev/kandev/pkg/pluginsdk"
	"github.com/stretchr/testify/require"

	"kandev-plugin-gitea/internal/gitea"
	sourcecontrol "kandev-plugin-gitea/recipes/source-control/server"
)

func pushLinkerFixture(t *testing.T, handler http.HandlerFunc) (*PushLinker, *fakeHost) {
	t.Helper()
	ts := httptest.NewTLSServer(handler)
	t.Cleanup(ts.Close)
	host := newFakeHost()
	host.repos = []pluginsdk.Repository{
		{ID: "repo-1", WorkspaceID: "ws-1", Name: "backend", ProviderID: "gitea", ProviderRepositoryID: "77", OwnerOrProject: "org", ProviderName: "backend"},
		{ID: "repo-2", WorkspaceID: "ws-1", Name: "other", ProviderID: "github", ProviderRepositoryID: "88", OwnerOrProject: "org", ProviderName: "other"},
		{ID: "repo-3", WorkspaceID: "ws-1", Name: "detached", ProviderID: "gitea", ProviderRepositoryID: "99", OwnerOrProject: "org", ProviderName: "detached"},
	}
	host.tasks = []pluginsdk.Task{{
		ID: "task-1", WorkspaceID: "ws-1",
		Repositories: []pluginsdk.TaskRepository{{RepositoryID: "repo-1"}, {RepositoryID: "repo-2"}},
	}}
	linker := NewPushLinker(host)
	linker.RetryDelays = []time.Duration{0, 0}
	linker.ClientFactory = func(context.Context, string) (*gitea.Client, *ConnectionState, error) {
		c, err := gitea.NewClient(ts.URL, "token")
		require.NoError(t, err)
		c.SetHTTPClient(ts.Client())
		return c, &ConnectionState{BaseURL: "https://gitea.example.com", Login: "bot"}, nil
	}
	return linker, host
}

func writePRs(w http.ResponseWriter, prs []gitea.PullRequest) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(prs)
}

func TestLinkAfterPushLinksMatchingOpenPR(t *testing.T) {
	var paths []string
	linker, host := pushLinkerFixture(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		writePRs(w, []gitea.PullRequest{
			{Number: 3, Head: gitea.PRBranch{Ref: "other"}},
			{Number: 5, Head: gitea.PRBranch{Ref: "feature/x"}},
		})
	})
	require.NoError(t, linker.LinkAfterPush(context.Background(), "task-1", "", "feature/x"))
	require.Equal(t, []string{"/api/v1/repos/org/backend/pulls"}, paths)

	linked, err := linker.Associations.ListForTask(context.Background(), "task-1")
	require.NoError(t, err)
	require.Equal(t, []sourcecontrol.ChangeRequestIdentity{{
		ConnectionScope: "https://gitea.example.com|bot", RepositoryID: "77", Number: 5,
	}}, linked)
	_ = host
}

func TestLinkAfterPushSkipsAlreadyLinked(t *testing.T) {
	linker, _ := pushLinkerFixture(t, func(w http.ResponseWriter, r *http.Request) {
		writePRs(w, []gitea.PullRequest{{Number: 5, Head: gitea.PRBranch{Ref: "feature/x"}}})
	})
	identity := sourcecontrol.ChangeRequestIdentity{ConnectionScope: "https://gitea.example.com|bot", RepositoryID: "77", Number: 5}
	require.NoError(t, linker.Associations.Link(context.Background(), "task-1", identity))
	require.NoError(t, linker.LinkAfterPush(context.Background(), "task-1", "", "feature/x"))
	linked, err := linker.Associations.ListForTask(context.Background(), "task-1")
	require.NoError(t, err)
	require.Len(t, linked, 1)
}

func TestLinkAfterPushRetriesUntilPRExists(t *testing.T) {
	var calls atomic.Int32
	linker, _ := pushLinkerFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			writePRs(w, nil)
			return
		}
		writePRs(w, []gitea.PullRequest{{Number: 9, Head: gitea.PRBranch{Ref: "feature/x"}}})
	})
	require.NoError(t, linker.LinkAfterPush(context.Background(), "task-1", "", "feature/x"))
	require.Equal(t, int32(3), calls.Load())
	linked, err := linker.Associations.ListForTask(context.Background(), "task-1")
	require.NoError(t, err)
	require.Len(t, linked, 1)
	require.Equal(t, int64(9), linked[0].Number)
}

func TestLinkAfterPushGivesUpAfterRetries(t *testing.T) {
	var calls atomic.Int32
	linker, _ := pushLinkerFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		writePRs(w, nil)
	})
	require.NoError(t, linker.LinkAfterPush(context.Background(), "task-1", "", "feature/x"))
	require.Equal(t, int32(3), calls.Load())
	linked, err := linker.Associations.ListForTask(context.Background(), "task-1")
	require.NoError(t, err)
	require.Empty(t, linked)
}

func TestLinkAfterPushIgnoresUnknownRepositoryName(t *testing.T) {
	var calls atomic.Int32
	linker, _ := pushLinkerFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		writePRs(w, nil)
	})
	require.NoError(t, linker.LinkAfterPush(context.Background(), "task-1", "detached", "feature/x"))
	require.Equal(t, int32(0), calls.Load())
}

func TestLinkAfterPushMatchesRepositoryName(t *testing.T) {
	var paths []string
	linker, _ := pushLinkerFixture(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		writePRs(w, []gitea.PullRequest{{Number: 1, Head: gitea.PRBranch{Ref: "feature/x"}}})
	})
	require.NoError(t, linker.LinkAfterPush(context.Background(), "task-1", "backend-branch2", "feature/x"))
	require.Equal(t, []string{"/api/v1/repos/org/backend/pulls"}, paths)
}

func TestLinkAfterPushRequiresBranchAndTask(t *testing.T) {
	linker, _ := pushLinkerFixture(t, func(w http.ResponseWriter, r *http.Request) { writePRs(w, nil) })
	require.Error(t, linker.LinkAfterPush(context.Background(), "task-1", "", ""))
	require.Error(t, linker.LinkAfterPush(context.Background(), "missing", "", "feature/x"))
}

func TestLinkAfterPushStopsOnContextCancel(t *testing.T) {
	linker, _ := pushLinkerFixture(t, func(w http.ResponseWriter, r *http.Request) { writePRs(w, nil) })
	linker.RetryDelays = []time.Duration{time.Minute}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := linker.LinkAfterPush(ctx, "task-1", "", "feature/x")
	require.ErrorIs(t, err, context.Canceled)
}
