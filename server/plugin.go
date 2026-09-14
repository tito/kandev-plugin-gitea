package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/kandev/kandev/pkg/pluginsdk"
	giteaplugin "kandev-plugin-gitea/internal/plugin"
	sourcecontrol "kandev-plugin-gitea/recipes/source-control/server"
)

const (
	providerID      = "gitea"
	referenceSource = "gitea_pull_requests"
)

type giteaPlugin struct {
	pluginsdk.UnimplementedPlugin
	sourcecontrol.Extension
}

var (
	_ pluginsdk.Plugin                  = (*giteaPlugin)(nil)
	_ pluginsdk.ActionHandler           = (*giteaPlugin)(nil)
	_ pluginsdk.EntityReferenceSearcher = (*giteaPlugin)(nil)
	_ pluginsdk.GitCredentialHandler    = (*giteaPlugin)(nil)
)

func newPlugin() *giteaPlugin {
	p := &giteaPlugin{}
	p.Extension = sourcecontrol.Extension{
		ProviderID:      providerID,
		ReferenceSource: referenceSource,
	}
	return p
}

func (p *giteaPlugin) wireAdapters() {
	host := p.Host()
	if host == nil {
		return
	}
	p.Extension.Repositories = &giteaplugin.RepositoryLister{Host: host}
	p.Extension.RepositoryDetails = &giteaplugin.RepositoryDetailsProvider{Host: host}
	p.Extension.AttachedRepositories = &giteaplugin.AttachedRepositoryResolver{Host: host}
	p.Extension.ChangeRequests = &giteaplugin.ChangeRequestService{Host: host}
	p.Extension.Associations = &giteaplugin.AssociationStore{Host: host}
	p.Extension.Reviews = &giteaplugin.ReviewReader{Host: host}
}

func (p *giteaPlugin) ensureWired() {
	if p.Extension.Repositories == nil {
		p.wireAdapters()
	}
}

func (p *giteaPlugin) OnEvent(_ context.Context, e *pluginsdk.Event) error {
	log.Printf("gitea plugin: event type=%s id=%s", e.EventType, e.EventID)
	p.ensureWired()
	return nil
}

func (p *giteaPlugin) HandleAction(ctx context.Context, req *pluginsdk.PluginActionRequest) (*pluginsdk.PluginActionResponse, error) {
	p.ensureWired()
	switch req.ActionKey {
	case "connection.get":
		return p.handleConnectionGet(ctx, req)
	case "connection.save":
		return p.handleConnectionSave(ctx, req)
	case "connection.disconnect":
		return p.handleConnectionDisconnect(ctx, req)
	default:
		return p.Extension.HandleAction(ctx, req)
	}
}

func (p *giteaPlugin) ResolveGitCredential(ctx context.Context, req *pluginsdk.ResolveGitCredentialRequest) (*pluginsdk.ResolveGitCredentialResponse, error) {
	host := p.Host()
	if host == nil {
		return nil, fmt.Errorf("gitea: host not available")
	}
	return giteaplugin.ResolveGitCredential(ctx, host, req)
}

func (p *giteaPlugin) GetGitCredentialBinding(ctx context.Context, req *pluginsdk.GitCredentialBindingRequest) (*pluginsdk.GitCredentialBindingResponse, error) {
	host := p.Host()
	if host == nil {
		return nil, fmt.Errorf("gitea: host not available")
	}
	return giteaplugin.GetGitCredentialBinding(ctx, host, req)
}

func (p *giteaPlugin) handleConnectionGet(ctx context.Context, req *pluginsdk.PluginActionRequest) (*pluginsdk.PluginActionResponse, error) {
	wsID := strings.TrimSpace(req.Context.WorkspaceID)
	if wsID == "" {
		return nil, fmt.Errorf("gitea: workspace is required")
	}
	host := p.Host()
	if host == nil {
		return actionJSON(map[string]any{"connected": false})
	}
	conn, err := giteaplugin.GetConnection(ctx, host, wsID)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return actionJSON(map[string]any{"connected": false})
	}
	return actionJSON(map[string]any{
		"connected": true,
		"base_url":  conn.BaseURL,
		"login":     conn.Login,
	})
}

func (p *giteaPlugin) handleConnectionSave(ctx context.Context, req *pluginsdk.PluginActionRequest) (*pluginsdk.PluginActionResponse, error) {
	wsID := strings.TrimSpace(req.Context.WorkspaceID)
	if wsID == "" {
		return nil, fmt.Errorf("gitea: workspace is required")
	}
	host := p.Host()
	if host == nil {
		return nil, fmt.Errorf("gitea: host not available")
	}
	var input struct {
		BaseURL string `json:"base_url"`
		PAT     string `json:"pat"`
	}
	if err := json.Unmarshal(req.Body, &input); err != nil {
		return nil, fmt.Errorf("gitea: decode connection input: %w", err)
	}
	conn, err := giteaplugin.SaveConnection(ctx, host, wsID, input.BaseURL, input.PAT)
	if err != nil {
		return actionJSON(map[string]any{"connected": false, "error": err.Error()})
	}
	return actionJSON(map[string]any{
		"connected": true,
		"base_url":  conn.BaseURL,
		"login":     conn.Login,
	})
}

func (p *giteaPlugin) handleConnectionDisconnect(ctx context.Context, req *pluginsdk.PluginActionRequest) (*pluginsdk.PluginActionResponse, error) {
	wsID := strings.TrimSpace(req.Context.WorkspaceID)
	if wsID == "" {
		return nil, fmt.Errorf("gitea: workspace is required")
	}
	host := p.Host()
	if host == nil {
		return actionJSON(map[string]any{"disconnected": true})
	}
	if err := giteaplugin.Disconnect(ctx, host, wsID); err != nil {
		return nil, err
	}
	return actionJSON(map[string]any{"disconnected": true})
}

func actionJSON(value any) (*pluginsdk.PluginActionResponse, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("gitea plugin: encode action response: %w", err)
	}
	return &pluginsdk.PluginActionResponse{
		Body:    body,
		Headers: map[string]string{"Content-Type": "application/json"},
	}, nil
}
