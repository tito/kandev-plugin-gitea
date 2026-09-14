package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/kandev/kandev/pkg/pluginsdk"
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
)

func newPlugin() *giteaPlugin {
	return &giteaPlugin{
		Extension: sourcecontrol.Extension{
			ProviderID:      providerID,
			ReferenceSource: referenceSource,
		},
	}
}

func (p *giteaPlugin) OnEvent(_ context.Context, e *pluginsdk.Event) error {
	log.Printf("gitea plugin: event type=%s id=%s", e.EventType, e.EventID)
	return nil
}

func (p *giteaPlugin) HandleAction(ctx context.Context, req *pluginsdk.PluginActionRequest) (*pluginsdk.PluginActionResponse, error) {
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

func (p *giteaPlugin) handleConnectionGet(_ context.Context, _ *pluginsdk.PluginActionRequest) (*pluginsdk.PluginActionResponse, error) {
	return actionJSON(map[string]any{"connected": false})
}

func (p *giteaPlugin) handleConnectionSave(_ context.Context, _ *pluginsdk.PluginActionRequest) (*pluginsdk.PluginActionResponse, error) {
	return actionJSON(map[string]any{"connected": false, "error": "not implemented"})
}

func (p *giteaPlugin) handleConnectionDisconnect(_ context.Context, _ *pluginsdk.PluginActionRequest) (*pluginsdk.PluginActionResponse, error) {
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
