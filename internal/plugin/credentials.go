package plugin

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/kandev/kandev/pkg/pluginsdk"
)

const credentialTTLDuration = 5 * time.Minute

func ResolveGitCredential(ctx context.Context, host pluginsdk.Host, req *pluginsdk.ResolveGitCredentialRequest) (*pluginsdk.ResolveGitCredentialResponse, error) {
	if err := validateCredentialScope(ctx, host, req); err != nil {
		return nil, err
	}
	conn, err := GetConnection(ctx, host, req.WorkspaceID)
	if err != nil || conn == nil {
		return nil, fmt.Errorf("gitea: no connection for workspace %s", req.WorkspaceID)
	}
	if err := matchCredentialOrigin(conn.BaseURL, req.Host, req.Path); err != nil {
		return nil, err
	}
	pat, found, err := host.GetSecret(ctx, patSecretKey(req.WorkspaceID))
	if err != nil || !found || pat == "" {
		return nil, fmt.Errorf("gitea: PAT not available for credential lease")
	}
	return &pluginsdk.ResolveGitCredentialResponse{
		Username:  conn.Login,
		Secret:    pat,
		ExpiresAt: time.Now().Add(credentialTTLDuration).UTC().Format(time.RFC3339),
	}, nil
}

func GetGitCredentialBinding(ctx context.Context, host pluginsdk.Host, req *pluginsdk.GitCredentialBindingRequest) (*pluginsdk.GitCredentialBindingResponse, error) {
	conn, err := GetConnection(ctx, host, req.WorkspaceID)
	if err != nil || conn == nil {
		return nil, fmt.Errorf("gitea: no connection for workspace %s", req.WorkspaceID)
	}
	binding := fmt.Sprintf("gitea-credential:%d", conn.Generation)
	return &pluginsdk.GitCredentialBindingResponse{Binding: binding}, nil
}

func validateCredentialScope(ctx context.Context, host pluginsdk.Host, req *pluginsdk.ResolveGitCredentialRequest) error {
	if req.WorkspaceID == "" || req.TaskID == "" || req.RepositoryID == "" {
		return fmt.Errorf("gitea: credential lease requires workspace, task, and repository")
	}
	repos := host.Repositories()
	repoList, _, err := repos.List(ctx, req.WorkspaceID, pluginsdk.Page{Limit: 500})
	if err != nil {
		return fmt.Errorf("gitea: list repositories: %w", err)
	}
	found := false
	for _, r := range repoList {
		if r.ID == req.RepositoryID && r.ProviderID == "gitea" {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("gitea: repository %s is not a gitea-provider repository in workspace %s", req.RepositoryID, req.WorkspaceID)
	}
	return nil
}

func matchCredentialOrigin(baseURL, leaseHost, leasePath string) error {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("gitea: invalid base URL: %w", err)
	}
	expectedHost := parsed.Host
	if !strings.EqualFold(expectedHost, leaseHost) {
		return fmt.Errorf("gitea: credential lease host %q does not match connection %q", leaseHost, expectedHost)
	}
	basePath := strings.TrimSuffix(parsed.Path, "/")
	if basePath != "" && !strings.HasPrefix("/"+leasePath, basePath) {
		return fmt.Errorf("gitea: credential lease path %q does not match connection path prefix %q", leasePath, basePath)
	}
	return nil
}
