package docker

import (
	"context"
	"gdocker/models"
	"strings"
	"time"

	"github.com/moby/moby/client"
)

func LoadContainers(m *models.Model) error {
	containerList, err := m.DockerClient.ContainerList(context.Background(), client.ContainerListOptions{All: true})
	if err != nil {
		return err
	}
	// Parse containers
	var containers []models.Container
	for _, c := range containerList.Items {
		name := strings.TrimPrefix(c.Names[0], "/")
		project := c.Labels["com.docker.compose.project"]

		// Parse ports
		var ports []models.PortMapping
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

		// Get environment variables by inspecting the container
		var env []string
		if inspect, err := m.DockerClient.ContainerInspect(context.Background(), c.ID, client.ContainerInspectOptions{}); err == nil {
			env = inspect.Container.Config.Env
		}

		containers = append(containers, models.Container{
			ID:      c.ID[:12],
			Name:    name,
			Image:   c.Image,
			State:   string(c.State),
			Status:  c.Status,
			Project: project,
			Created: time.Unix(c.Created, 0),
			Ports:   ports,
			Env:     env,
		})
	}

	// Group by compose projects
	m.Containers = containers
	m.Standalone, m.Projects = GroupByProject(containers)
	RebuildItems(m)

	return nil
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

	var projects []models.ComposeGroup
	for name, containers := range projectMap {
		projects = append(projects, models.ComposeGroup{
			Name:       name,
			Containers: containers,
			Expanded:   false,
		})
	}

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
	volumeList, err := m.DockerClient.VolumeList(context.Background(), client.VolumeListOptions{})
	if err != nil {
		return err
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

	m.Volumes = volumes
	return nil
}

func LoadImages(m *models.Model) error {
	imageList, err := m.DockerClient.ImageList(context.Background(), client.ImageListOptions{All: true})
	if err != nil {
		return err
	}

	var images []models.Image
	for _, img := range imageList.Items {
		images = append(images, models.Image{
			ID:       img.ID[7:19], // Short ID (remove "sha256:" prefix and truncate)
			RepoTags: img.RepoTags,
			Size:     img.Size,
			Created:  time.Unix(img.Created, 0),
		})
	}

	m.Images = images
	return nil
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
	networkList, err := m.DockerClient.NetworkList(context.Background(), client.NetworkListOptions{})
	if err != nil {
		return err
	}

	var networks []models.Network
	for _, n := range networkList.Items {
		networks = append(networks, models.Network{
			ID:       n.ID[:12], // Short ID
			Name:     n.Name,
			Driver:   n.Driver,
			Scope:    n.Scope,
			Created:  n.Created,
			Internal: n.Internal,
			Labels:   n.Labels,
		})
	}

	m.Networks = networks
	return nil
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
