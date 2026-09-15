package plugin

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/kandev/kandev/pkg/pluginsdk"
	"kandev-plugin-gitea/internal/gitea"
	sourcecontrol "kandev-plugin-gitea/recipes/source-control/server"
)

var defaultPushRetryDelays = []time.Duration{30 * time.Second, 60 * time.Second}

type PushLinker struct {
	Host          pluginsdk.Host
	Associations  *AssociationStore
	ClientFactory func(ctx context.Context, workspaceID string) (*gitea.Client, *ConnectionState, error)
	RetryDelays   []time.Duration
}

func NewPushLinker(host pluginsdk.Host) *PushLinker {
	return &PushLinker{
		Host:         host,
		Associations: &AssociationStore{Host: host},
		RetryDelays:  defaultPushRetryDelays,
	}
}

// LinkAfterPush links the open pull request whose head is branch to the task, retrying while none exists yet.
func (l *PushLinker) LinkAfterPush(ctx context.Context, taskID, repositoryName, branch string) error {
	if branch == "" {
		return fmt.Errorf("gitea: push branch is required")
	}
	task, err := l.Host.Tasks().Get(ctx, taskID)
	if err != nil {
		return fmt.Errorf("gitea: get task: %w", err)
	}
	if task == nil || task.WorkspaceID == "" {
		return fmt.Errorf("gitea: task %s has no workspace", taskID)
	}
	factory := l.ClientFactory
	if factory == nil {
		factory = func(ctx context.Context, workspaceID string) (*gitea.Client, *ConnectionState, error) {
			return ClientForWorkspace(ctx, l.Host, workspaceID)
		}
	}
	client, conn, err := factory(ctx, task.WorkspaceID)
	if err != nil {
		return err
	}
	repos, err := l.candidateRepositories(ctx, task, repositoryName)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		return nil
	}
	scope := ConnectionScope(conn.BaseURL, conn.Login)
	for attempt := 0; ; attempt++ {
		linked, err := l.linkOnce(ctx, client, scope, taskID, repos, branch)
		if err != nil {
			return err
		}
		if linked || attempt >= len(l.RetryDelays) {
			return nil
		}
		if err := sleepContext(ctx, l.RetryDelays[attempt]); err != nil {
			return err
		}
	}
}

func (l *PushLinker) candidateRepositories(ctx context.Context, task *pluginsdk.Task, repositoryName string) ([]pluginsdk.Repository, error) {
	all, _, err := l.Host.Repositories().List(ctx, task.WorkspaceID, pluginsdk.Page{Limit: 500})
	if err != nil {
		return nil, fmt.Errorf("gitea: list repositories: %w", err)
	}
	attached := make(map[string]bool, len(task.Repositories))
	for _, tr := range task.Repositories {
		attached[tr.RepositoryID] = true
	}
	var candidates []pluginsdk.Repository
	for _, repo := range all {
		if repo.ProviderID != "gitea" || !attached[repo.ID] {
			continue
		}
		if repo.OwnerOrProject == "" || repo.ProviderName == "" || repo.ProviderRepositoryID == "" {
			continue
		}
		candidates = append(candidates, repo)
	}
	if repositoryName == "" {
		return candidates, nil
	}
	for _, repo := range candidates {
		if repo.Name == repositoryName {
			return []pluginsdk.Repository{repo}, nil
		}
	}
	for _, repo := range candidates {
		if strings.HasPrefix(repositoryName, repo.Name+"-") {
			return []pluginsdk.Repository{repo}, nil
		}
	}
	return nil, nil
}

func (l *PushLinker) linkOnce(ctx context.Context, client *gitea.Client, scope, taskID string, repos []pluginsdk.Repository, branch string) (bool, error) {
	for _, repo := range repos {
		pr, found, err := findOpenPullRequest(ctx, client, repo.OwnerOrProject, repo.ProviderName, branch)
		if err != nil {
			return false, err
		}
		if !found {
			continue
		}
		identity := sourcecontrol.ChangeRequestIdentity{
			ConnectionScope: scope,
			RepositoryID:    repo.ProviderRepositoryID,
			Number:          pr.Number,
		}
		existing, err := l.Associations.ListForTask(ctx, taskID)
		if err != nil {
			return false, err
		}
		for _, e := range existing {
			if e == identity {
				return true, nil
			}
		}
		if err := l.Associations.Link(ctx, taskID, identity); err != nil {
			return false, fmt.Errorf("gitea: link pull request: %w", err)
		}
		log.Printf("gitea plugin: linked pull request %s/%s#%d to task %s after push", repo.OwnerOrProject, repo.ProviderName, pr.Number, taskID)
		return true, nil
	}
	return false, nil
}

func findOpenPullRequest(ctx context.Context, client *gitea.Client, owner, name, branch string) (gitea.PullRequest, bool, error) {
	page := 1
	for {
		result, err := client.ListPullRequests(ctx, owner, name, "open", page, 50)
		if err != nil {
			return gitea.PullRequest{}, false, fmt.Errorf("gitea: list pull requests: %w", err)
		}
		for _, pr := range result.Items {
			if pr.Head.Ref == branch {
				return pr, true, nil
			}
		}
		if result.NextPage == 0 || page >= 4 {
			return gitea.PullRequest{}, false, nil
		}
		page = result.NextPage
	}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
