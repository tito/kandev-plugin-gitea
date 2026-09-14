package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kandev/kandev/pkg/pluginsdk"
	"kandev-plugin-gitea/internal/gitea"
)

const (
	connectionStateKey = "connection"
	patSecretPrefix    = "gitea_pat_"
)

type ConnectionState struct {
	BaseURL    string `json:"base_url"`
	Login      string `json:"login"`
	Generation int    `json:"generation"`
}

func ConnectionScope(baseURL, login string) string {
	return strings.TrimSuffix(baseURL, "/") + "|" + login
}

func GetConnection(ctx context.Context, host pluginsdk.Host, workspaceID string) (*ConnectionState, error) {
	value, found, err := host.GetState(ctx, "workspace", workspaceID, connectionStateKey)
	if err != nil {
		return nil, fmt.Errorf("gitea: read connection state: %w", err)
	}
	if !found {
		return nil, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("gitea: encode connection state: %w", err)
	}
	var conn ConnectionState
	if err := json.Unmarshal(data, &conn); err != nil {
		return nil, fmt.Errorf("gitea: decode connection state: %w", err)
	}
	if conn.BaseURL == "" || conn.Login == "" {
		return nil, nil
	}
	return &conn, nil
}

func SaveConnection(ctx context.Context, host pluginsdk.Host, workspaceID, baseURL, pat string) (*ConnectionState, error) {
	baseURL = strings.TrimSpace(baseURL)
	pat = strings.TrimSpace(pat)
	if baseURL == "" || pat == "" {
		return nil, fmt.Errorf("gitea: base URL and personal access token are required")
	}

	client, err := gitea.NewClient(baseURL, pat)
	if err != nil {
		return nil, err
	}
	user, err := client.GetCurrentUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("gitea: verify connection: %w", err)
	}

	existing, _ := GetConnection(ctx, host, workspaceID)
	generation := 1
	if existing != nil {
		generation = existing.Generation + 1
	}

	secretKey := patSecretKey(workspaceID)
	if err := host.SetSecret(ctx, secretKey, pat); err != nil {
		return nil, fmt.Errorf("gitea: store PAT: %w", err)
	}

	conn := ConnectionState{
		BaseURL:    client.BaseURL(),
		Login:      user.Login,
		Generation: generation,
	}
	stateValue := map[string]any{
		"base_url":   conn.BaseURL,
		"login":      conn.Login,
		"generation": conn.Generation,
	}
	if err := host.SetState(ctx, "workspace", workspaceID, connectionStateKey, stateValue); err != nil {
		return nil, fmt.Errorf("gitea: store connection state: %w", err)
	}
	return &conn, nil
}

func Disconnect(ctx context.Context, host pluginsdk.Host, workspaceID string) error {
	if err := host.DeleteSecret(ctx, patSecretKey(workspaceID)); err != nil {
		return fmt.Errorf("gitea: delete PAT: %w", err)
	}
	if err := host.DeleteState(ctx, "workspace", workspaceID, connectionStateKey); err != nil {
		return fmt.Errorf("gitea: delete connection state: %w", err)
	}
	return nil
}

func ClientForWorkspace(ctx context.Context, host pluginsdk.Host, workspaceID string) (*gitea.Client, *ConnectionState, error) {
	conn, err := GetConnection(ctx, host, workspaceID)
	if err != nil {
		return nil, nil, err
	}
	if conn == nil {
		return nil, nil, fmt.Errorf("gitea: no connection configured for this workspace")
	}
	pat, found, err := host.GetSecret(ctx, patSecretKey(workspaceID))
	if err != nil {
		return nil, nil, fmt.Errorf("gitea: read PAT: %w", err)
	}
	if !found || pat == "" {
		return nil, nil, fmt.Errorf("gitea: PAT not found for this workspace")
	}
	client, err := gitea.NewClient(conn.BaseURL, pat)
	if err != nil {
		return nil, nil, err
	}
	return client, conn, nil
}

func patSecretKey(workspaceID string) string {
	return patSecretPrefix + workspaceID
}
