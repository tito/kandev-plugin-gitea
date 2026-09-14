package plugin

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/kandev/kandev/pkg/pluginsdk"
	"kandev-plugin-gitea/internal/gitea"
	sourcecontrol "kandev-plugin-gitea/recipes/source-control/server"
)

type RepositoryLister struct {
	Host pluginsdk.Host
}

func (l *RepositoryLister) ConnectionScope(ctx context.Context, workspaceID string) (string, error) {
	conn, err := GetConnection(ctx, l.Host, workspaceID)
	if err != nil || conn == nil {
		return "", fmt.Errorf("gitea: no connection configured")
	}
	return ConnectionScope(conn.BaseURL, conn.Login), nil
}

func (l *RepositoryLister) List(ctx context.Context, workspaceID, query string, cursor sourcecontrol.RepositoryCursor, limit int) (sourcecontrol.RepositoryPage, error) {
	client, conn, err := ClientForWorkspace(ctx, l.Host, workspaceID)
	if err != nil {
		return sourcecontrol.RepositoryPage{}, err
	}
	page := 1
	if cursor.AfterRepositoryID != "" {
		p, err := strconv.Atoi(cursor.Remote)
		if err == nil && p > 0 {
			page = p
		}
	}
	result, err := client.SearchRepositories(ctx, query, page, limit)
	if err != nil {
		return sourcecontrol.RepositoryPage{}, fmt.Errorf("gitea: search repositories: %w", err)
	}
	scope := ConnectionScope(conn.BaseURL, conn.Login)
	repos := make([]sourcecontrol.Repository, 0, len(result.Items))
	for _, r := range result.Items {
		repos = append(repos, toSourceControlRepo(r, conn.BaseURL, scope))
	}
	var nextCursor sourcecontrol.RepositoryCursor
	if result.NextPage > 0 {
		lastID := ""
		if len(result.Items) > 0 {
			lastID = strconv.FormatInt(result.Items[len(result.Items)-1].ID, 10)
		}
		nextCursor = sourcecontrol.RepositoryCursor{
			Remote:            strconv.Itoa(result.NextPage),
			AfterRepositoryID: lastID,
		}
	}
	return sourcecontrol.RepositoryPage{Repositories: repos, Next: nextCursor}, nil
}

type RepositoryDetailsProvider struct {
	Host pluginsdk.Host
}

func (d *RepositoryDetailsProvider) Inspect(ctx context.Context, workspaceID, rawURL string) (*sourcecontrol.Repository, error) {
	client, conn, err := ClientForWorkspace(ctx, d.Host, workspaceID)
	if err != nil {
		return nil, nil
	}
	owner, name := parseGiteaRepoURL(conn.BaseURL, rawURL)
	if owner == "" || name == "" {
		return nil, nil
	}
	repo, err := client.GetRepository(ctx, owner, name)
	if err != nil {
		return nil, nil
	}
	scope := ConnectionScope(conn.BaseURL, conn.Login)
	result := toSourceControlRepo(repo, conn.BaseURL, scope)
	return &result, nil
}

func (d *RepositoryDetailsProvider) Resolve(ctx context.Context, workspaceID string, identity sourcecontrol.RepositoryIdentity) (sourcecontrol.Repository, error) {
	client, conn, err := ClientForWorkspace(ctx, d.Host, workspaceID)
	if err != nil {
		return sourcecontrol.Repository{}, err
	}
	repoID, err := strconv.ParseInt(identity.RepositoryID, 10, 64)
	if err != nil {
		return sourcecontrol.Repository{}, fmt.Errorf("gitea: invalid repository ID %q", identity.RepositoryID)
	}
	repos, searchErr := client.SearchRepositories(ctx, "", 1, 100)
	if searchErr != nil {
		return sourcecontrol.Repository{}, fmt.Errorf("gitea: search repositories: %w", searchErr)
	}
	for _, r := range repos.Items {
		if r.ID == repoID {
			scope := ConnectionScope(conn.BaseURL, conn.Login)
			return toSourceControlRepo(r, conn.BaseURL, scope), nil
		}
	}
	return sourcecontrol.Repository{}, fmt.Errorf("gitea: repository %d not found", repoID)
}

func (d *RepositoryDetailsProvider) ListBranches(ctx context.Context, workspaceID string, repository sourcecontrol.Repository) ([]sourcecontrol.Branch, error) {
	client, _, err := ClientForWorkspace(ctx, d.Host, workspaceID)
	if err != nil {
		return nil, err
	}
	owner, name := ownerAndName(repository)
	if owner == "" || name == "" {
		return nil, fmt.Errorf("gitea: repository owner and name are required")
	}
	var branches []sourcecontrol.Branch
	page := 1
	for {
		result, err := client.ListBranches(ctx, owner, name, page, 50)
		if err != nil {
			return nil, fmt.Errorf("gitea: list branches: %w", err)
		}
		for _, b := range result.Items {
			branches = append(branches, sourcecontrol.Branch{
				Name:      b.Name,
				Commit:    b.Commit.ID,
				IsDefault: b.Name == repository.DefaultBranch,
			})
		}
		if result.NextPage == 0 {
			break
		}
		page = result.NextPage
	}
	return branches, nil
}

type AttachedRepositoryResolver struct {
	Host pluginsdk.Host
}

func (r *AttachedRepositoryResolver) ResolveAttached(ctx context.Context, action pluginsdk.VerifiedActionContext) (sourcecontrol.Repository, error) {
	repos := r.Host.Repositories()
	repoList, _, err := repos.List(ctx, action.WorkspaceID, pluginsdk.Page{Limit: 500})
	if err != nil {
		return sourcecontrol.Repository{}, fmt.Errorf("gitea: list repositories: %w", err)
	}
	for _, repo := range repoList {
		if repo.ID == action.RepositoryID && repo.ProviderID == "gitea" {
			conn, err := GetConnection(ctx, r.Host, action.WorkspaceID)
			if err != nil || conn == nil {
				return sourcecontrol.Repository{}, fmt.Errorf("gitea: no connection for workspace")
			}
			scope := ConnectionScope(conn.BaseURL, conn.Login)
			return sourcecontrol.Repository{
				ProviderID:      "gitea",
				ProviderHost:    conn.BaseURL,
				ConnectionScope: scope,
				RepositoryID:    repo.ProviderRepositoryID,
				OwnerOrProject:  repo.OwnerOrProject,
				Name:            repo.ProviderName,
				CloneURL:        repo.RemoteURL,
				DefaultBranch:   derefString(repo.DefaultBranch),
			}, nil
		}
	}
	return sourcecontrol.Repository{}, fmt.Errorf("gitea: repository %s not attached to task", action.RepositoryID)
}

func toSourceControlRepo(r gitea.Repository, baseURL, scope string) sourcecontrol.Repository {
	return sourcecontrol.Repository{
		ProviderID:      "gitea",
		ProviderHost:    strings.TrimSuffix(baseURL, "/"),
		ConnectionScope: scope,
		RepositoryID:    strconv.FormatInt(r.ID, 10),
		OwnerOrProject:  r.Owner.Login,
		Name:            r.Name,
		CloneURL:        r.CloneURL,
		DefaultBranch:   r.DefaultBranch,
	}
}

func ownerAndName(repo sourcecontrol.Repository) (string, string) {
	return repo.OwnerOrProject, repo.Name
}

func parseGiteaRepoURL(baseURL, rawURL string) (owner, name string) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", ""
	}
	baseParsed, err := url.Parse(baseURL)
	if err != nil {
		return "", ""
	}
	if !strings.EqualFold(parsed.Host, baseParsed.Host) {
		return "", ""
	}
	path := strings.TrimPrefix(parsed.Path, baseParsed.Path)
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, ".git")
	path = strings.TrimSuffix(path, "/")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", ""
	}
	return parts[0], parts[1]
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
