package models

import (
	"github.com/moby/moby/client"
)

// NewModel creates a new Model with sensible defaults using composition
func NewModel(dockerClient *client.Client) Model {
	return Model{
		DockerClient: dockerClient,
		NavMode:      NavContainers,
		ViewMode:     ViewDetails,
		Width:        0,
		Height:       0,
		Cursor:       0,
		Containers:   []Container{},
		Standalone:   []Container{},
		Projects:     []ComposeGroup{},
		Items:        []ListItem{},
		Volumes:      []Volume{},
		Images:       []Image{},
		Networks:     []Network{},
	}
}

// Future: NewModelV2 for the refactored version
func NewModelV2(dockerClient *client.Client) ModelV2 {
	model := ModelV2{
		DockerClient: dockerClient,
		Docker: &DockerState{
			Containers: []Container{},
			Standalone: []Container{},
			Projects:   []ComposeGroup{},
			Volumes:    []Volume{},
			Images:     []Image{},
			Networks:   []Network{},
		},
		UI: &UIState{
			Navigation: NavigationState{
				Mode:  NavContainers,
				Items: []ListItem{},
			},
			View: ViewState{
				Mode: ViewDetails,
				Logs: &LogsState{
					Lines:     []string{},
					ScrollPos: 0,
					Following: false,
					Search:    nil,
				},
				Ports: &PortsState{
					Selected: 0,
				},
				Inspect: &InspectState{
					Data:      "",
					ScrollPos: 0,
				},
				VolumeBrowse: &VolumeState{
					Files: []string{},
					Path:  "",
				},
			},
			Dimensions: DimensionState{
				Width:  0,
				Height: 0,
			},
			List: ListState{
				Cursor: 0,
			},
			Message: "",
		},
	}

	// Initialize ResourceState with references
	model.Resources = &ResourceState{
		docker: model.Docker,
		nav:    &model.UI.Navigation,
	}

	return model
}
