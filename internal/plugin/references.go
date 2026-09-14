package plugin

import (
	"context"
	"fmt"
	"strconv"

	"github.com/kandev/kandev/pkg/pluginsdk"
)

type ReferenceService struct {
	Host pluginsdk.Host
}

func (s *ReferenceService) Search(ctx context.Context, workspaceID, query string, limit int) ([]pluginsdk.EntityReferenceCandidate, error) {
	client, conn, err := ClientForWorkspace(ctx, s.Host, workspaceID)
	if err != nil {
		return nil, err
	}
	scope := ConnectionScope(conn.BaseURL, conn.Login)

	prs, err := client.SearchRepositories(ctx, query, 1, 10)
	if err != nil {
		return nil, fmt.Errorf("gitea: search repos for references: %w", err)
	}

	var candidates []pluginsdk.EntityReferenceCandidate
	for _, repo := range prs.Items {
		prPage, err := client.ListPullRequests(ctx, repo.Owner.Login, repo.Name, "open", 1, limit)
		if err != nil {
			continue
		}
		for _, pr := range prPage.Items {
			repoIDStr := strconv.FormatInt(repo.ID, 10)
			localID := fmt.Sprintf("%s/%s/%d", scope, repoIDStr, pr.Number)
			candidates = append(candidates, pluginsdk.EntityReferenceCandidate{
				ProviderLocalID: localID,
				Title:           pr.Title,
				URL:             pr.HTMLURL,
				Attributes: map[string]any{
					"connection_scope": scope,
					"repository_id":    repoIDStr,
					"number":           float64(pr.Number),
				},
			})
			if len(candidates) >= limit {
				return candidates, nil
			}
		}
	}
	return candidates, nil
}

func (s *ReferenceService) Authorize(ctx context.Context, workspaceID, _ string, reference map[string]any) (bool, error) {
	client, _, err := ClientForWorkspace(ctx, s.Host, workspaceID)
	if err != nil {
		return false, nil
	}

	repoID, _ := reference["repository_id"].(string)
	numberFloat, _ := reference["number"].(float64)
	number := int64(numberFloat)
	if repoID == "" || number <= 0 {
		return false, nil
	}

	repos := s.Host.Repositories()
	repoList, _, err := repos.List(ctx, workspaceID, pluginsdk.Page{Limit: 500})
	if err != nil {
		return false, nil
	}
	var owner, name string
	for _, r := range repoList {
		if r.ProviderRepositoryID == repoID && r.ProviderID == "gitea" {
			owner = r.OwnerOrProject
			name = r.ProviderName
			break
		}
	}
	if owner == "" || name == "" {
		return false, nil
	}

	_, err = client.GetPullRequest(ctx, owner, name, number)
	return err == nil, nil
}
