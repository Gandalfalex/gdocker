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

// listContainers lists all containers; prev keeps environments already
// loaded.
func (c *Client) listContainers(prev []models.Container) ([]models.Container, error) {
	containerList, err := c.cli.ContainerList(context.Background(), client.ContainerListOptions{All: true})
	if err != nil {
		return nil, err
	}
	return containersFromList(containerList.Items, envCacheFromContainers(prev)), nil
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
