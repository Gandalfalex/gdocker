package app

import (
	"fmt"

	"gdocker/config"
	"gdocker/docker"
	"gdocker/models"
	"gdocker/ui/logviewer"
	tk "github.com/Gandalfalex/tuikit"
)

// New loads the configuration, connects to Docker and loads the lists.
func New() (Model, error) {
	appConfig, configErr := config.Load()
	if configErr != nil {
		// If config loading fails, use defaults (and say so below).
		appConfig = config.DefaultAppConfig()
	}

	// Configured host overrides DOCKER_HOST.
	cli, err := docker.Connect(appConfig.Docker.Host)
	if err != nil {
		return Model{}, err
	}
	snap, err := cli.Load()
	if err != nil {
		cli.Close()
		return Model{}, err
	}

	m := Model{
		KeyBindings:     &appConfig.KeyBindings,
		UIConfig:        &appConfig.UI,
		Backend:         cli,
		DockerHost:      appConfig.Docker.Host,
		HostAliases:     appConfig.Docker.HostAliases,
		ViewMode:        models.ViewDetails,
		NavMode:         models.NavContainers,
		AutoRefreshSecs: appConfig.Docker.AutoRefreshSeconds,
		LogTailLines:    appConfig.Docker.LogTailLines,
		LogViewer:       logviewer.New(&appConfig.KeyBindings.Logs, appConfig.UI.ShowLineNumbers, appConfig.UI.WrapLogs, tk.Copy),
		Containers:      snap.Containers,
		Volumes:         snap.Volumes,
		Images:          snap.Images,
		Networks:        snap.Networks,
	}
	m.Standalone, m.Projects = groupByProject(m.Containers)
	rebuildContainerItems(&m)
	if configErr != nil {
		m.startup = fmt.Sprintf("Config ignored, using defaults: %v", configErr)
	}
	return m, nil
}
