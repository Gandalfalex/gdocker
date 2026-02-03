package models

import (
	"github.com/moby/moby/client"
)

// Model is the root application state using composition
type ModelV2 struct {
	// Core components
	Docker    *DockerState
	UI        *UIState
	Resources *ResourceState

	// Docker client
	DockerClient *client.Client
}

// DockerState holds all Docker-related data
type DockerState struct {
	Containers []Container
	Standalone []Container
	Projects   []ComposeGroup
	Volumes    []Volume
	Images     []Image
	Networks   []Network
}

// UIState holds all UI-related state
type UIState struct {
	Navigation NavigationState
	View       ViewState
	Dimensions DimensionState
	List       ListState
	Message    string
}

// NavigationState tracks which resource type we're viewing
type NavigationState struct {
	Mode  NavigationMode
	Items []ListItem
}

// ViewState tracks the current view and its specific state
type ViewState struct {
	Mode         ViewMode
	Logs         *LogsState
	Ports        *PortsState
	Stats        *ContainerStats
	Inspect      *InspectState
	VolumeBrowse *VolumeState
}

// LogsState holds log viewing state
type LogsState struct {
	Lines     []string
	ScrollPos int
	Following bool
	Search    *SearchState
}

// SearchState holds search-specific state
type SearchState struct {
	Active      bool
	Query       string
	Results     []int
	ResultIndex int
}

// PortsState holds port viewing state
type PortsState struct {
	Selected int
}

// InspectState holds inspect viewing state
type InspectState struct {
	Data      string
	ScrollPos int
}

// VolumeState holds volume browsing state
type VolumeState struct {
	Files []string
	Path  string
}

// DimensionState holds terminal dimensions
type DimensionState struct {
	Width  int
	Height int
}

// ListState holds list navigation state
type ListState struct {
	Cursor int
}

// ResourceState provides access to current resource
type ResourceState struct {
	docker *DockerState
	nav    *NavigationState
}

// GetCurrentContainer returns the currently selected container, if any
func (r *ResourceState) GetCurrentContainer() *Container {
	if r.nav.Mode != NavContainers {
		return nil
	}

	for _, item := range r.nav.Items {
		if item.IsContainer {
			return item.Container
		}
	}
	return nil
}

// GetCurrentVolume returns the currently selected volume, if any
func (r *ResourceState) GetCurrentVolume() *Volume {
	if r.nav.Mode != NavVolumes {
		return nil
	}

	for _, item := range r.nav.Items {
		if item.IsVolume {
			return item.Volume
		}
	}
	return nil
}

// GetCurrentImage returns the currently selected image, if any
func (r *ResourceState) GetCurrentImage() *Image {
	if r.nav.Mode != NavImages {
		return nil
	}

	for _, item := range r.nav.Items {
		if item.IsImage {
			return item.Image
		}
	}
	return nil
}

// GetCurrentNetwork returns the currently selected network, if any
func (r *ResourceState) GetCurrentNetwork() *Network {
	if r.nav.Mode != NavNetworks {
		return nil
	}

	for _, item := range r.nav.Items {
		if item.IsNetwork {
			return item.Network
		}
	}
	return nil
}
