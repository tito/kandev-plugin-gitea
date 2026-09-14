package gitea

import "time"

type User struct {
	ID       int64  `json:"id"`
	Login    string `json:"login"`
	FullName string `json:"full_name"`
	Email    string `json:"email"`
}

type Repository struct {
	ID            int64  `json:"id"`
	Owner         User   `json:"owner"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	CloneURL      string `json:"clone_url"`
	HTMLURL       string `json:"html_url"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
	Fork          bool   `json:"fork"`
	Archived      bool   `json:"archived"`

	AllowMerge  bool `json:"allow_merge_commits"`
	AllowRebase bool `json:"allow_rebase"`
	AllowSquash bool `json:"allow_squash_merge"`
}

type Branch struct {
	Name   string       `json:"name"`
	Commit BranchCommit `json:"commit"`
}

type BranchCommit struct {
	ID string `json:"id"`
}

type PullRequest struct {
	ID        int64      `json:"id"`
	Number    int64      `json:"number"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	State     string     `json:"state"`
	HTMLURL   string     `json:"html_url"`
	Draft     bool       `json:"draft"`
	Merged    bool       `json:"merged"`
	MergedAt  *time.Time `json:"merged_at"`
	ClosedAt  *time.Time `json:"closed_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	User      User       `json:"user"`
	Head      PRBranch   `json:"head"`
	Base      PRBranch   `json:"base"`
	Comments  int        `json:"comments"`
}

type PRBranch struct {
	Label string `json:"label"`
	Ref   string `json:"ref"`
	SHA   string `json:"sha"`
}

type CreatePullRequestOption struct {
	Head  string `json:"head"`
	Base  string `json:"base"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

type MergePullRequestOption struct {
	Do string `json:"Do"`
}

type Review struct {
	ID          int64     `json:"id"`
	User        User      `json:"user"`
	State       string    `json:"state"`
	Body        string    `json:"body"`
	CommitID    string    `json:"commit_id"`
	SubmittedAt time.Time `json:"submitted_at"`
}

type CreateReviewOption struct {
	Event string `json:"event"`
	Body  string `json:"body"`
}

type Comment struct {
	ID        int64     `json:"id"`
	User      User      `json:"user"`
	Body      string    `json:"body"`
	HTMLURL   string    `json:"html_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateCommentOption struct {
	Body string `json:"body"`
}

type CombinedStatus struct {
	State    string         `json:"state"`
	Statuses []CommitStatus `json:"statuses"`
	SHA      string         `json:"sha"`
}

type CommitStatus struct {
	ID          int64  `json:"id"`
	State       string `json:"state"`
	Context     string `json:"context"`
	Description string `json:"description"`
	TargetURL   string `json:"target_url"`
}

type ServerVersion struct {
	Version string `json:"version"`
}

type APIError struct {
	Message string `json:"message"`
	URL     string `json:"url"`
}
