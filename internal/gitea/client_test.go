package gitea

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateBaseURL_RejectsHTTP(t *testing.T) {
	_, err := NewClient("http://gitea.example.com", "token")
	require.ErrorContains(t, err, "HTTPS")
}

func TestValidateBaseURL_RejectsEmptyHost(t *testing.T) {
	_, err := NewClient("https://", "token")
	require.ErrorContains(t, err, "host")
}

func TestValidateBaseURL_RejectsCredentialsInURL(t *testing.T) {
	_, err := NewClient("https://user:pass@gitea.example.com", "token")
	require.ErrorContains(t, err, "credentials")
}

func TestValidateBaseURL_AcceptsPathPrefix(t *testing.T) {
	c, err := NewClient("https://example.com/gitea", "token")
	require.NoError(t, err)
	require.Equal(t, "https://example.com/gitea", c.BaseURL())
}

func TestValidateBaseURL_StripsQueryAndFragment(t *testing.T) {
	c, err := NewClient("https://example.com/gitea?foo=bar#frag", "token")
	require.NoError(t, err)
	require.Equal(t, "https://example.com/gitea", c.BaseURL())
}

func testServer(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	ts := httptest.NewTLSServer(handler)
	t.Cleanup(ts.Close)
	c, err := NewClient(ts.URL, "test-token")
	require.NoError(t, err)
	c.httpClient = ts.Client()
	return c, ts
}

func TestGetVersion(t *testing.T) {
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/version", r.URL.Path)
		require.Equal(t, "token test-token", r.Header.Get("Authorization"))
		json.NewEncoder(w).Encode(ServerVersion{Version: "1.24.0"})
	})
	v, err := c.GetVersion(context.Background())
	require.NoError(t, err)
	require.Equal(t, "1.24.0", v.Version)
}

func TestGetCurrentUser(t *testing.T) {
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/user", r.URL.Path)
		json.NewEncoder(w).Encode(User{ID: 1, Login: "testuser"})
	})
	u, err := c.GetCurrentUser(context.Background())
	require.NoError(t, err)
	require.Equal(t, "testuser", u.Login)
}

func TestErrorMapping_401(t *testing.T) {
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(APIError{Message: "token is invalid"})
	})
	_, err := c.GetCurrentUser(context.Background())
	require.Error(t, err)
	var giteaErr *Error
	require.ErrorAs(t, err, &giteaErr)
	require.Equal(t, ErrUnauthenticated, giteaErr.Code)
}

func TestErrorMapping_403(t *testing.T) {
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		json.NewEncoder(w).Encode(APIError{Message: "access denied"})
	})
	_, err := c.GetCurrentUser(context.Background())
	var giteaErr *Error
	require.ErrorAs(t, err, &giteaErr)
	require.Equal(t, ErrPermissionDenied, giteaErr.Code)
}

func TestErrorMapping_404(t *testing.T) {
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})
	_, err := c.GetRepository(context.Background(), "owner", "repo")
	var giteaErr *Error
	require.ErrorAs(t, err, &giteaErr)
	require.Equal(t, ErrNotFound, giteaErr.Code)
}

func TestErrorMapping_429(t *testing.T) {
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
	})
	_, err := c.GetCurrentUser(context.Background())
	var giteaErr *Error
	require.ErrorAs(t, err, &giteaErr)
	require.Equal(t, ErrRateLimited, giteaErr.Code)
}

func TestErrorMapping_500(t *testing.T) {
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(APIError{Message: "internal error"})
	})
	_, err := c.GetCurrentUser(context.Background())
	var giteaErr *Error
	require.ErrorAs(t, err, &giteaErr)
	require.Equal(t, ErrUpstream, giteaErr.Code)
}

func TestSearchRepositories_Pagination(t *testing.T) {
	callCount := 0
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		require.Equal(t, "widgets", r.URL.Query().Get("q"))
		require.Equal(t, "updated", r.URL.Query().Get("sort"))
		page := r.URL.Query().Get("page")
		w.Header().Set("X-Total-Count", "3")
		if page == "1" {
			json.NewEncoder(w).Encode([]Repository{
				{ID: 1, Name: "widget-a", FullName: "org/widget-a"},
				{ID: 2, Name: "widget-b", FullName: "org/widget-b"},
			})
		} else {
			json.NewEncoder(w).Encode([]Repository{
				{ID: 3, Name: "widget-c", FullName: "org/widget-c"},
			})
		}
	})

	page1, err := c.SearchRepositories(context.Background(), "widgets", 1, 2)
	require.NoError(t, err)
	require.Len(t, page1.Items, 2)
	require.Equal(t, 3, page1.TotalCount)
	require.Equal(t, 2, page1.NextPage)

	page2, err := c.SearchRepositories(context.Background(), "widgets", 2, 2)
	require.NoError(t, err)
	require.Len(t, page2.Items, 1)
	require.Equal(t, 0, page2.NextPage)
	require.Equal(t, 2, callCount)
}

func TestCreatePullRequest_DraftTitlePrefix(t *testing.T) {
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v1/repos/org/repo/pulls", r.URL.Path)
		var opt CreatePullRequestOption
		json.NewDecoder(r.Body).Decode(&opt)
		require.Equal(t, "WIP: my change", opt.Title)
		require.Equal(t, "feature-branch", opt.Head)
		require.Equal(t, "main", opt.Base)
		json.NewEncoder(w).Encode(PullRequest{
			Number:  42,
			Title:   "WIP: my change",
			HTMLURL: "https://gitea.example.com/org/repo/pulls/42",
			Draft:   true,
		})
	})
	pr, err := c.CreatePullRequest(context.Background(), "org", "repo", CreatePullRequestOption{
		Head:  "feature-branch",
		Base:  "main",
		Title: "WIP: my change",
	})
	require.NoError(t, err)
	require.Equal(t, int64(42), pr.Number)
	require.True(t, pr.Draft)
}

func TestGetPullRequest_MergedDetection(t *testing.T) {
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/repos/org/repo/pulls/42", r.URL.Path)
		json.NewEncoder(w).Encode(PullRequest{
			Number: 42,
			State:  "closed",
			Merged: true,
		})
	})
	pr, err := c.GetPullRequest(context.Background(), "org", "repo", 42)
	require.NoError(t, err)
	require.Equal(t, "closed", pr.State)
	require.True(t, pr.Merged)
}

func TestListReviews(t *testing.T) {
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/repos/org/repo/pulls/42/reviews", r.URL.Path)
		json.NewEncoder(w).Encode([]Review{
			{ID: 1, State: "APPROVED", User: User{Login: "reviewer1"}},
			{ID: 2, State: "REQUEST_CHANGES", User: User{Login: "reviewer2"}},
		})
	})
	page, err := c.ListReviews(context.Background(), "org", "repo", 42, 1, 50)
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	require.Equal(t, "APPROVED", page.Items[0].State)
	require.Equal(t, "REQUEST_CHANGES", page.Items[1].State)
}

func TestGetCombinedStatus(t *testing.T) {
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/repos/org/repo/commits/abc123/status", r.URL.Path)
		json.NewEncoder(w).Encode(CombinedStatus{
			State: "failure",
			SHA:   "abc123",
			Statuses: []CommitStatus{
				{ID: 1, Context: "ci/build", State: "success", TargetURL: "https://ci.example/1"},
				{ID: 2, Context: "ci/test", State: "failure", Description: "Tests failed"},
			},
		})
	})
	cs, err := c.GetCombinedStatus(context.Background(), "org", "repo", "abc123")
	require.NoError(t, err)
	require.Equal(t, "failure", cs.State)
	require.Len(t, cs.Statuses, 2)
	require.Equal(t, "success", cs.Statuses[0].State)
	require.Equal(t, "failure", cs.Statuses[1].State)
}

func TestMergePullRequest(t *testing.T) {
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v1/repos/org/repo/pulls/42/merge", r.URL.Path)
		var opt MergePullRequestOption
		json.NewDecoder(r.Body).Decode(&opt)
		require.Equal(t, "squash", opt.Do)
		w.WriteHeader(200)
	})
	err := c.MergePullRequest(context.Background(), "org", "repo", 42, MergePullRequestOption{Do: "squash"})
	require.NoError(t, err)
}

func TestCancelledContext(t *testing.T) {
	c, _ := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request should not have been sent")
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.GetVersion(ctx)
	require.Error(t, err)
}

func TestBaseURLWithPathPrefix(t *testing.T) {
	c, ts := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/gitea/api/v1/version", r.URL.Path)
		json.NewEncoder(w).Encode(ServerVersion{Version: "1.24.0"})
	})
	// Override the base URL to use a path prefix
	c.baseURL.Path = "/gitea/"
	_ = ts
	v, err := c.GetVersion(context.Background())
	require.NoError(t, err)
	require.Equal(t, "1.24.0", v.Version)
}
