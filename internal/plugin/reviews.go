package plugin

import (
	"context"
	"fmt"
	"strconv"

	"github.com/kandev/kandev/pkg/pluginsdk"
	sourcecontrol "kandev-plugin-gitea/recipes/source-control/server"
)

type ReviewReader struct {
	Host pluginsdk.Host
}

func (r *ReviewReader) ForTask(ctx context.Context, workspaceID, taskID string) ([]sourcecontrol.ReviewSummary, error) {
	assocStore := &AssociationStore{Host: r.Host}
	associations, err := assocStore.ListForTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if len(associations) == 0 {
		return nil, nil
	}

	client, conn, err := ClientForWorkspace(ctx, r.Host, workspaceID)
	if err != nil {
		return nil, err
	}

	var summaries []sourcecontrol.ReviewSummary
	for _, assoc := range associations {
		owner, name, err := resolveOwnerName(ctx, r.Host, workspaceID, assoc.RepositoryID)
		if err != nil {
			continue
		}
		pr, err := client.GetPullRequest(ctx, owner, name, assoc.Number)
		if err != nil {
			continue
		}

		state := "open"
		if pr.Merged {
			state = "merged"
		} else if pr.State == "closed" {
			state = "closed"
		}
		if pr.Draft && state == "open" {
			state = "draft"
		}

		pipelineState := "neutral"
		var checks []sourcecontrol.ReviewTaskStatusCheck
		if pr.Head.SHA != "" {
			cs, err := client.GetCombinedStatus(ctx, owner, name, pr.Head.SHA)
			if err == nil {
				pipelineState = mapCombinedState(cs.State)
				for _, s := range cs.Statuses {
					checks = append(checks, sourcecontrol.ReviewTaskStatusCheck{
						ID:     strconv.FormatInt(s.ID, 10),
						Label:  s.Context,
						State:  mapStatusState(s.State),
						Detail: s.Description,
						URL:    s.TargetURL,
					})
				}
			}
		}

		var review *sourcecontrol.ReviewTaskReview
		reviews, err := client.ListReviews(ctx, owner, name, pr.Number, 1, 50)
		if err == nil {
			approved := 0
			changesRequested := 0
			for _, rev := range reviews.Items {
				switch rev.State {
				case "APPROVED":
					approved++
				case "REQUEST_CHANGES":
					changesRequested++
				}
			}
			reviewState := "pending"
			if changesRequested > 0 {
				reviewState = "changes_requested"
			} else if approved > 0 {
				reviewState = "approved"
			}
			review = &sourcecontrol.ReviewTaskReview{
				State:    reviewState,
				Approved: approved,
			}
		}

		scope := ConnectionScope(conn.BaseURL, conn.Login)
		reviewKey := fmt.Sprintf("%s/%s#%d", owner, name, pr.Number)

		summaries = append(summaries, sourcecontrol.ReviewSummary{
			ProviderID:          "gitea",
			ReviewKey:           reviewKey,
			Title:               pr.Title,
			URL:                 pr.HTMLURL,
			ConnectionScope:     scope,
			RepositoryID:        assoc.RepositoryID,
			ChangeRequestNumber: pr.Number,
			State:               state,
			TaskStatus: &sourcecontrol.ReviewTaskStatus{
				Number:             pr.Number,
				State:              state,
				PipelineState:      pipelineState,
				Checks:             checks,
				Review:             review,
				UnresolvedComments: pr.Comments,
				UpdatedAt:          pr.UpdatedAt.UnixMilli(),
			},
		})
	}
	return summaries, nil
}

func (r *ReviewReader) Associations(ctx context.Context, workspaceID string) ([]sourcecontrol.ReviewAssociation, error) {
	assocStore := &AssociationStore{Host: r.Host}
	tasks := r.Host.Tasks()
	taskList, _, err := tasks.List(ctx, pluginsdk.TaskFilter{WorkspaceIDs: []string{workspaceID}}, pluginsdk.Page{Limit: 500})
	if err != nil {
		return nil, fmt.Errorf("gitea: list tasks: %w", err)
	}
	var associations []sourcecontrol.ReviewAssociation
	for _, task := range taskList {
		taskAssocs, err := assocStore.ListForTask(ctx, task.ID)
		if err != nil {
			continue
		}
		for _, assoc := range taskAssocs {
			owner, name, err := resolveOwnerName(ctx, r.Host, workspaceID, assoc.RepositoryID)
			if err != nil {
				continue
			}
			reviewKey := fmt.Sprintf("%s/%s#%d", owner, name, assoc.Number)
			associations = append(associations, sourcecontrol.ReviewAssociation{
				ProviderID:          "gitea",
				TaskID:              task.ID,
				ReviewKey:           reviewKey,
				ConnectionScope:     assoc.ConnectionScope,
				RepositoryID:        assoc.RepositoryID,
				ChangeRequestNumber: assoc.Number,
			})
		}
	}
	return associations, nil
}

func resolveOwnerName(ctx context.Context, host pluginsdk.Host, workspaceID, providerRepoID string) (string, string, error) {
	repos := host.Repositories()
	repoList, _, err := repos.List(ctx, workspaceID, pluginsdk.Page{Limit: 500})
	if err != nil {
		return "", "", err
	}
	for _, r := range repoList {
		if r.ProviderRepositoryID == providerRepoID && r.ProviderID == "gitea" {
			return r.OwnerOrProject, r.ProviderName, nil
		}
	}
	return "", "", fmt.Errorf("gitea: repository %s not found", providerRepoID)
}

func mapCombinedState(state string) string {
	switch state {
	case "success":
		return "success"
	case "pending":
		return "pending"
	case "failure", "error":
		return "failure"
	default:
		return "neutral"
	}
}

func mapStatusState(state string) string {
	switch state {
	case "success":
		return "success"
	case "pending":
		return "pending"
	case "failure", "error":
		return "failure"
	default:
		return "neutral"
	}
}

