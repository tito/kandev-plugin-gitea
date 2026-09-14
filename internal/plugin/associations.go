package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kandev/kandev/pkg/pluginsdk"
	sourcecontrol "kandev-plugin-gitea/recipes/source-control/server"
)

const associationStatePrefix = "pr_assoc_"

type AssociationStore struct {
	Host pluginsdk.Host
}

func (s *AssociationStore) Link(ctx context.Context, taskID string, identity sourcecontrol.ChangeRequestIdentity) error {
	key := associationKey(identity)
	value := map[string]any{
		"connection_scope": identity.ConnectionScope,
		"repository_id":    identity.RepositoryID,
		"number":           identity.Number,
	}
	return s.Host.SetState(ctx, "task", taskID, key, value)
}

func (s *AssociationStore) Unlink(ctx context.Context, taskID string, identity sourcecontrol.ChangeRequestIdentity) error {
	key := associationKey(identity)
	return s.Host.DeleteState(ctx, "task", taskID, key)
}

func (s *AssociationStore) ListForTask(ctx context.Context, taskID string) ([]sourcecontrol.ChangeRequestIdentity, error) {
	entries, err := s.Host.ListState(ctx, "task", taskID)
	if err != nil {
		return nil, fmt.Errorf("gitea: list task state: %w", err)
	}
	var results []sourcecontrol.ChangeRequestIdentity
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Key, associationStatePrefix) {
			continue
		}
		data, err := json.Marshal(entry.Value)
		if err != nil {
			continue
		}
		var identity sourcecontrol.ChangeRequestIdentity
		if err := json.Unmarshal(data, &identity); err != nil {
			continue
		}
		if identity.ConnectionScope != "" && identity.RepositoryID != "" && identity.Number > 0 {
			results = append(results, identity)
		}
	}
	return results, nil
}

func associationKey(identity sourcecontrol.ChangeRequestIdentity) string {
	return fmt.Sprintf("%s%s_%s_%d", associationStatePrefix, identity.ConnectionScope, identity.RepositoryID, identity.Number)
}
