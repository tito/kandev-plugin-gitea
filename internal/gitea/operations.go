package gitea

import (
	"context"
	"fmt"
	"net/url"
)

func (c *Client) GetVersion(ctx context.Context) (ServerVersion, error) {
	var v ServerVersion
	return v, c.get(ctx, "version", nil, &v)
}

func (c *Client) GetCurrentUser(ctx context.Context) (User, error) {
	var u User
	return u, c.get(ctx, "user", nil, &u)
}

func (c *Client) SearchRepositories(ctx context.Context, query string, page, limit int) (Page[Repository], error) {
	q := url.Values{"sort": {"updated"}}
	if query != "" {
		q.Set("q", query)
	}
	return getPage[Repository](ctx, c, "repos/search", q, page, limit)
}

func (c *Client) GetRepository(ctx context.Context, owner, repo string) (Repository, error) {
	var r Repository
	return r, c.get(ctx, fmt.Sprintf("repos/%s/%s", owner, repo), nil, &r)
}

func (c *Client) ListBranches(ctx context.Context, owner, repo string, page, limit int) (Page[Branch], error) {
	return getPage[Branch](ctx, c, fmt.Sprintf("repos/%s/%s/branches", owner, repo), nil, page, limit)
}

func (c *Client) CreatePullRequest(ctx context.Context, owner, repo string, opt CreatePullRequestOption) (PullRequest, error) {
	var pr PullRequest
	return pr, c.post(ctx, fmt.Sprintf("repos/%s/%s/pulls", owner, repo), opt, &pr)
}

func (c *Client) ListPullRequests(ctx context.Context, owner, repo, state string, page, limit int) (Page[PullRequest], error) {
	q := url.Values{}
	if state != "" {
		q.Set("state", state)
	}
	return getPage[PullRequest](ctx, c, fmt.Sprintf("repos/%s/%s/pulls", owner, repo), q, page, limit)
}

func (c *Client) GetPullRequest(ctx context.Context, owner, repo string, number int64) (PullRequest, error) {
	var pr PullRequest
	return pr, c.get(ctx, fmt.Sprintf("repos/%s/%s/pulls/%d", owner, repo, number), nil, &pr)
}

func (c *Client) MergePullRequest(ctx context.Context, owner, repo string, number int64, opt MergePullRequestOption) error {
	return c.post(ctx, fmt.Sprintf("repos/%s/%s/pulls/%d/merge", owner, repo, number), opt, nil)
}

func (c *Client) UpdatePullRequest(ctx context.Context, owner, repo string, number int64, fields map[string]any) (PullRequest, error) {
	var pr PullRequest
	return pr, c.patch(ctx, fmt.Sprintf("repos/%s/%s/pulls/%d", owner, repo, number), fields, &pr)
}

func (c *Client) ListReviews(ctx context.Context, owner, repo string, number int64, page, limit int) (Page[Review], error) {
	return getPage[Review](ctx, c, fmt.Sprintf("repos/%s/%s/pulls/%d/reviews", owner, repo, number), nil, page, limit)
}

func (c *Client) CreateReview(ctx context.Context, owner, repo string, number int64, opt CreateReviewOption) (Review, error) {
	var r Review
	return r, c.post(ctx, fmt.Sprintf("repos/%s/%s/pulls/%d/reviews", owner, repo, number), opt, &r)
}

func (c *Client) DeleteReview(ctx context.Context, owner, repo string, number, reviewID int64) error {
	return c.delete(ctx, fmt.Sprintf("repos/%s/%s/pulls/%d/reviews/%d", owner, repo, number, reviewID))
}

func (c *Client) ListIssueComments(ctx context.Context, owner, repo string, number int64, page, limit int) (Page[Comment], error) {
	return getPage[Comment](ctx, c, fmt.Sprintf("repos/%s/%s/issues/%d/comments", owner, repo, number), nil, page, limit)
}

func (c *Client) CreateIssueComment(ctx context.Context, owner, repo string, number int64, opt CreateCommentOption) (Comment, error) {
	var comment Comment
	return comment, c.post(ctx, fmt.Sprintf("repos/%s/%s/issues/%d/comments", owner, repo, number), opt, &comment)
}

func (c *Client) GetCombinedStatus(ctx context.Context, owner, repo, ref string) (CombinedStatus, error) {
	var cs CombinedStatus
	return cs, c.get(ctx, fmt.Sprintf("repos/%s/%s/commits/%s/status", owner, repo, ref), nil, &cs)
}
