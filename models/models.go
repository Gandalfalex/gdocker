package models

import (
	"gdocker/config"
	"time"

	"github.com/moby/moby/client"
)

type Model struct {
	KeyBindings     *config.KeyBindings
	UIConfig        *config.UIConfig
	Containers      []Container
	Standalone      []Container
	Projects        []ComposeGroup
	Items           []ListItem
	Cursor          int
	SelectedPort    int // For port selection to open in browser
	Width           int
	Height          int
	NavMode         NavigationMode
	ViewMode        ViewMode
	Logs            []string
	LogScroll       int
	StatusMessage   string
	DockerClient    *client.Client
	SearchMode      bool   // Whether we're in search input mode
	SearchQuery     string // Current search query
	SearchResults   []int  // Line indices that match the search
	SearchResultIdx int    // Current position in SearchResults
	CommandMode     bool   // Whether we're in command mode (:)
	CommandInput    string // Current command input
	HelpMode        bool   // Whether we're in help view
	Stats           *ContainerStats
	Volumes         []Volume
	Images          []Image
	Networks        []Network
	VolumeFiles     []string // Files in current volume directory
	VolumePath      string   // Current path in volume
	InspectData     string   // JSON inspect data
	FollowingLogs   bool     // Whether logs are being followed
}

type PortMapping struct {
	PrivatePort uint16
	PublicPort  uint16
	Type        string
	IP          string
}

type ContainerStats struct {
	CPUPerc     string
	MemUsage    string
	MemPerc     string
	NetIO       string
	BlockIO     string
	PIDs        string
	ContainerID string
}

// Container holds container info
type Container struct {
	ID      string
	Name    string
	Image   string
	State   string
	Status  string
	Project string
	Created time.Time
	Ports   []PortMapping
	Env     []string
}

// Volume holds volume info
type Volume struct {
	Name       string
	Driver     string
	Mountpoint string
	Created    time.Time
	Labels     map[string]string
	Scope      string
}

// Image holds image info
type Image struct {
	ID       string
	RepoTags []string
	Size     int64
	Created  time.Time
}

// Network holds network info
type Network struct {
	ID       string
	Name     string
	Driver   string
	Scope    string
	Created  time.Time
	Internal bool
	Labels   map[string]string
}

// ComposeGroup groups containers by project
type ComposeGroup struct {
	Name       string
	Containers []Container
	Expanded   bool
}

// ListItem represents an item in the sidebar
type ListItem struct {
	IsProject   bool
	IsContainer bool
	IsVolume    bool
	IsImage     bool
	IsNetwork   bool
	Project     *ComposeGroup
	Container   *Container
	Volume      *Volume
	Image       *Image
	Network     *Network
	Index       int // Index in the projects/containers array
}

// NavigationMode represents what we're viewing in the left panel
type NavigationMode int

const (
	NavContainers NavigationMode = iota
	NavVolumes
	NavImages
	NavNetworks
)

// ViewMode represents what's shown in the right panel
type ViewMode int

const (
	ViewDetails ViewMode = iota
	ViewLogs
	ViewPorts
	ViewEnv
	ViewStats
	ViewVolumeBrowse
	ViewInspect
)

// Messages
type ContainersRefreshedMsg struct {
	Containers []Container
}

type LogsLoadedMsg struct {
	Lines  []string
	Follow bool
}

type LogLineMsg struct {
	Line string
}

type ActionResultMsg struct {
	Message string
	Success bool
}

type StatsLoadedMsg struct {
	Stats *ContainerStats
}

type VolumesLoadedMsg struct {
	Volumes []Volume
}

type ImagesLoadedMsg struct {
	Images []Image
}

type NetworksLoadedMsg struct {
	Networks []Network
}

type InspectLoadedMsg struct {
	Data string
}

type VolumeFilesMsg struct {
	Files []string
	Path  string
}
