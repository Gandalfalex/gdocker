package docker

import (
	"context"
	"gdocker/models"
	"sort"
	"strings"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

func LoadContainers(m *models.Model) error {
	containerList, err := m.DockerClient.ContainerList(context.Background(), client.ContainerListOptions{All: true})
	if err != nil {
		return err
	}
	containers := containersFromList(containerList.Items, envCacheFromContainers(m.Containers))

	// Group by compose projects
	m.Containers = containers
	m.Standalone, m.Projects = GroupByProject(containers)
	RebuildItems(m)

	return nil
}

type cachedContainerEnv struct {
	loaded bool
	env    []string
}

func envCacheFromContainers(containers []models.Container) map[string]cachedContainerEnv {
	cache := make(map[string]cachedContainerEnv, len(containers))
	for _, c := range containers {
		id := c.FullID
		if id == "" {
			id = c.ID
		}
		if id == "" {
			continue
		}
		cache[id] = cachedContainerEnv{
			loaded: c.EnvLoaded,
			env:    c.Env,
		}
	}
	return cache
}

func containersFromList(items []container.Summary, envCache map[string]cachedContainerEnv) []models.Container {
	containers := make([]models.Container, 0, len(items))
	for _, c := range items {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		if name == "" {
			name = shortID(c.ID)
		}

		project := c.Labels["com.docker.compose.project"]

		ports := make([]models.PortMapping, 0, len(c.Ports))
		for _, p := range c.Ports {
			ip := ""
			if p.IP.IsValid() {
				ip = p.IP.String()
			}
			ports = append(ports, models.PortMapping{
				PrivatePort: p.PrivatePort,
				PublicPort:  p.PublicPort,
				Type:        p.Type,
				IP:          ip,
			})
		}

		cached := envCache[c.ID]
		containers = append(containers, models.Container{
			ID:        shortID(c.ID),
			FullID:    c.ID,
			Name:      name,
			Image:     c.Image,
			State:     string(c.State),
			Status:    c.Status,
			Project:   project,
			Created:   time.Unix(c.Created, 0),
			Ports:     ports,
			Env:       cached.env,
			EnvLoaded: cached.loaded,
		})
	}

	return containers
}

func GroupByProject(containers []models.Container) ([]models.Container, []models.ComposeGroup) {
	standalone := []models.Container{}
	projectMap := make(map[string][]models.Container)

	for _, c := range containers {
		if c.Project == "" {
			standalone = append(standalone, c)
		} else {
			projectMap[c.Project] = append(projectMap[c.Project], c)
		}
	}

	// Map iteration order is random; sort so the list does not reshuffle
	// on every refresh.
	byName := func(cs []models.Container) {
		sort.SliceStable(cs, func(i, j int) bool { return cs[i].Name < cs[j].Name })
	}
	byName(standalone)

	var projects []models.ComposeGroup
	for name, containers := range projectMap {
		byName(containers)
		projects = append(projects, models.ComposeGroup{
			Name:       name,
			Containers: containers,
			Expanded:   false,
		})
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })

	return standalone, projects
}

func RebuildItems(m *models.Model) {
	m.Items = []models.ListItem{}

	// Add standalone containers
	for i := range m.Standalone {
		m.Items = append(m.Items, models.ListItem{
			IsContainer: true,
			Container:   &m.Standalone[i],
		})
	}

	// Add compose projects
	for i := range m.Projects {
		m.Items = append(m.Items, models.ListItem{
			IsProject: true,
			Project:   &m.Projects[i],
			Index:     i,
		})

		// Add expanded containers
		if m.Projects[i].Expanded {
			for j := range m.Projects[i].Containers {
				m.Items = append(m.Items, models.ListItem{
					IsContainer: true,
					Container:   &m.Projects[i].Containers[j],
				})
			}
		}
	}

	// Ensure cursor is valid
	if m.Cursor >= len(m.Items) {
		m.Cursor = len(m.Items) - 1
	}
	if m.Cursor < 0 && len(m.Items) > 0 {
		m.Cursor = 0
	}
}

func LoadVolumes(m *models.Model) error {
	volumes, err := listVolumes(m.DockerClient)
	if err != nil {
		return err
	}
	m.Volumes = volumes
	return nil
}

func listVolumes(cli *client.Client) ([]models.Volume, error) {
	volumeList, err := cli.VolumeList(context.Background(), client.VolumeListOptions{})
	if err != nil {
		return nil, err
	}

	var volumes []models.Volume
	for _, v := range volumeList.Items {
		// Parse the CreatedAt timestamp
		created := time.Now()
		if v.CreatedAt != "" {
			if t, err := time.Parse(time.RFC3339Nano, v.CreatedAt); err == nil {
				created = t
			}
		}

		volumes = append(volumes, models.Volume{
			Name:       v.Name,
			Driver:     v.Driver,
			Mountpoint: v.Mountpoint,
			Created:    created,
			Labels:     v.Labels,
			Scope:      v.Scope,
		})
	}
	sort.Slice(volumes, func(i, j int) bool { return volumes[i].Name < volumes[j].Name })
	return volumes, nil
}

func LoadImages(m *models.Model) error {
	images, err := listImages(m.DockerClient)
	if err != nil {
		return err
	}
	m.Images = images
	return nil
}

func listImages(cli *client.Client) ([]models.Image, error) {
	imageList, err := cli.ImageList(context.Background(), client.ImageListOptions{All: true})
	if err != nil {
		return nil, err
	}

	var images []models.Image
	for _, img := range imageList.Items {
		images = append(images, models.Image{
			ID:       shortID(strings.TrimPrefix(img.ID, "sha256:")),
			RepoTags: img.RepoTags,
			Size:     img.Size,
			Created:  time.Unix(img.Created, 0),
		})
	}
	return images, nil
}

func RebuildVolumeItems(m *models.Model) {
	m.Items = []models.ListItem{}

	for i := range m.Volumes {
		m.Items = append(m.Items, models.ListItem{
			IsVolume: true,
			Volume:   &m.Volumes[i],
			Index:    i,
		})
	}

	// Ensure cursor is valid
	if m.Cursor >= len(m.Items) {
		m.Cursor = len(m.Items) - 1
	}
	if m.Cursor < 0 && len(m.Items) > 0 {
		m.Cursor = 0
	}
}

func RebuildImageItems(m *models.Model) {
	m.Items = []models.ListItem{}

	for i := range m.Images {
		m.Items = append(m.Items, models.ListItem{
			IsImage: true,
			Image:   &m.Images[i],
			Index:   i,
		})
	}

	// Ensure cursor is valid
	if m.Cursor >= len(m.Items) {
		m.Cursor = len(m.Items) - 1
	}
	if m.Cursor < 0 && len(m.Items) > 0 {
		m.Cursor = 0
	}
}

func LoadNetworks(m *models.Model) error {
	networks, err := listNetworks(m.DockerClient)
	if err != nil {
		return err
	}
	m.Networks = networks
	return nil
}

func listNetworks(cli *client.Client) ([]models.Network, error) {
	networkList, err := cli.NetworkList(context.Background(), client.NetworkListOptions{})
	if err != nil {
		return nil, err
	}

	var networks []models.Network
	for _, n := range networkList.Items {
		networks = append(networks, models.Network{
			ID:       shortID(n.ID),
			Name:     n.Name,
			Driver:   n.Driver,
			Scope:    n.Scope,
			Created:  n.Created,
			Internal: n.Internal,
			Labels:   n.Labels,
		})
	}
	sort.Slice(networks, func(i, j int) bool { return networks[i].Name < networks[j].Name })
	return networks, nil
}

func RebuildNetworkItems(m *models.Model) {
	m.Items = []models.ListItem{}

	for i := range m.Networks {
		m.Items = append(m.Items, models.ListItem{
			IsNetwork: true,
			Network:   &m.Networks[i],
			Index:     i,
		})
	}

	// Ensure cursor is valid
	if m.Cursor >= len(m.Items) {
		m.Cursor = len(m.Items) - 1
	}
	if m.Cursor < 0 && len(m.Items) > 0 {
		m.Cursor = 0
	}
}
