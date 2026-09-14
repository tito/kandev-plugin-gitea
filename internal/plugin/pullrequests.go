package plugin

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"kandev-plugin-gitea/internal/gitea"
	sourcecontrol "kandev-plugin-gitea/recipes/source-control/server"

	"github.com/kandev/kandev/pkg/pluginsdk"
)

type ChangeRequestService struct {
	Host pluginsdk.Host
}

func (s *ChangeRequestService) ResolveReference(ctx context.Context, workspaceID, reference string) (sourcecontrol.ChangeRequest, error) {
	client, conn, err := ClientForWorkspace(ctx, s.Host, workspaceID)
	if err != nil {
		return sourcecontrol.ChangeRequest{}, err
	}
	owner, repo, number, err := parseReference(reference)
	if err != nil {
		return sourcecontrol.ChangeRequest{}, err
	}
	pr, err := client.GetPullRequest(ctx, owner, repo, number)
	if err != nil {
		return sourcecontrol.ChangeRequest{}, fmt.Errorf("gitea: get pull request: %w", err)
	}
	repoObj, err := client.GetRepository(ctx, owner, repo)
	if err != nil {
		return sourcecontrol.ChangeRequest{}, fmt.Errorf("gitea: get repository: %w", err)
	}
	scope := ConnectionScope(conn.BaseURL, conn.Login)
	return sourcecontrol.ChangeRequest{
		Identity: sourcecontrol.ChangeRequestIdentity{
			ConnectionScope: scope,
			RepositoryID:    strconv.FormatInt(repoObj.ID, 10),
			Number:          pr.Number,
		},
		Title: pr.Title,
		URL:   pr.HTMLURL,
	}, nil
}

func (s *ChangeRequestService) Create(ctx context.Context, repository sourcecontrol.Repository, headBranch string, input sourcecontrol.CreateChangeRequestInput) (sourcecontrol.ChangeRequest, error) {
	wsID := s.resolveWorkspace(ctx, repository.ConnectionScope)
	if wsID == "" {
		return sourcecontrol.ChangeRequest{}, fmt.Errorf("gitea: cannot resolve workspace for connection scope %q", repository.ConnectionScope)
	}
	client, conn, err := ClientForWorkspace(ctx, s.Host, wsID)
	if err != nil {
		return sourcecontrol.ChangeRequest{}, fmt.Errorf("gitea: client not available: %w", err)
	}
	owner, name := ownerAndName(repository)
	if owner == "" || name == "" {
		return sourcecontrol.ChangeRequest{}, fmt.Errorf("gitea: repository owner and name are required")
	}
	base := input.Destination
	if base == "" {
		base = repository.DefaultBranch
	}
	if base == "" {
		base = "main"
	}
	title := input.Title
	if input.Draft {
		if !strings.HasPrefix(title, "WIP:") && !strings.HasPrefix(title, "WIP: ") {
			title = "WIP: " + title
		}
	}

	existingPRs, err := client.ListPullRequests(ctx, owner, name, "open", 1, 100)
	if err == nil {
		for _, pr := range existingPRs.Items {
			if pr.Head.Ref == headBranch && pr.Base.Ref == base {
				scope := ConnectionScope(conn.BaseURL, conn.Login)
				return sourcecontrol.ChangeRequest{
					Identity: sourcecontrol.ChangeRequestIdentity{
						ConnectionScope: scope,
						RepositoryID:    repository.RepositoryID,
						Number:          pr.Number,
					},
					Title: pr.Title,
					URL:   pr.HTMLURL,
				}, nil
			}
		}
	}

	pr, err := client.CreatePullRequest(ctx, owner, name, gitea.CreatePullRequestOption{
		Head:  headBranch,
		Base:  base,
		Title: title,
		Body:  input.Description,
	})
	if err != nil {
		return sourcecontrol.ChangeRequest{}, fmt.Errorf("gitea: create pull request: %w", err)
	}
	scope := ConnectionScope(conn.BaseURL, conn.Login)
	return sourcecontrol.ChangeRequest{
		Identity: sourcecontrol.ChangeRequestIdentity{
			ConnectionScope: scope,
			RepositoryID:    repository.RepositoryID,
			Number:          pr.Number,
		},
		Title: pr.Title,
		URL:   pr.HTMLURL,
	}, nil
}

func (s *ChangeRequestService) resolveWorkspace(ctx context.Context, connectionScope string) string {
	workspaces := s.Host.Workspaces()
	wsList, _, err := workspaces.List(ctx, pluginsdk.Page{Limit: 100})
	if err != nil {
		return ""
	}
	for _, ws := range wsList {
		conn, err := GetConnection(ctx, s.Host, ws.ID)
		if err != nil || conn == nil {
			continue
		}
		if ConnectionScope(conn.BaseURL, conn.Login) == connectionScope {
			return ws.ID
		}
	}
	return ""
}

func parseReference(reference string) (owner, repo string, number int64, err error) {
	reference = strings.TrimSpace(reference)
	idx := strings.LastIndex(reference, "#")
	if idx < 0 {
		return "", "", 0, fmt.Errorf("gitea: invalid reference %q: expected owner/repo#number", reference)
	}
	path := reference[:idx]
	numStr := reference[idx+1:]
	n, parseErr := strconv.ParseInt(numStr, 10, 64)
	if parseErr != nil || n <= 0 {
		return "", "", 0, fmt.Errorf("gitea: invalid PR number in %q", reference)
	}
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", 0, fmt.Errorf("gitea: invalid reference %q: expected owner/repo#number", reference)
	}
	return parts[0], parts[1], n, nil
}
