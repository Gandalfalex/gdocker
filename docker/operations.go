package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"gdocker/models"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

func init() {
	// Set function pointers to avoid circular imports
	models.GroupByProjectFunc = GroupByProject
	models.RebuildItemsFunc = RebuildItems
	models.RebuildVolumeItemsFunc = RebuildVolumeItems
	models.RebuildImageItemsFunc = RebuildImageItems
	models.RebuildNetworkItemsFunc = RebuildNetworkItems
	models.RefreshContainersFunc = RefreshContainers
	models.StartContainerFunc = StartContainer
	models.StopContainerFunc = StopContainer
	models.RestartContainerFunc = RestartContainer
	models.DeleteContainerFunc = DeleteContainer
	models.DeleteVolumeFunc = DeleteVolume
	models.DeleteImageFunc = DeleteImage
	models.LoadLogsFunc = LoadLogs
	models.LoadInspectFunc = LoadInspect
	models.ExecShellFunc = ExecShell
	models.OpenPortInBrowserFunc = OpenPortInBrowser
	models.QuitFunc = Quit
	models.LoadStatsFunc = LoadStats
}

func RefreshContainers(m *models.Model) tea.Cmd {
	return func() tea.Msg {
		containerList, err := m.DockerClient.ContainerList(context.Background(), client.ContainerListOptions{All: true})
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Error: %v", err), Success: false}
		}

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

		return models.ContainersRefreshedMsg{Containers: containers}
	}
}

func StartContainer(m *models.Model) tea.Cmd {
	if m.Cursor >= len(m.Items) || !m.Items[m.Cursor].IsContainer {
		return nil
	}

	containerID := m.Items[m.Cursor].Container.ID

	return func() tea.Msg {
		_, err := m.DockerClient.ContainerStart(context.Background(), containerID, client.ContainerStartOptions{})
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed to start: %v", err), Success: false}
		}
		return models.ActionResultMsg{Message: "Container started", Success: true}
	}
}

func StopContainer(m *models.Model) tea.Cmd {
	if m.Cursor >= len(m.Items) || !m.Items[m.Cursor].IsContainer {
		return nil
	}

	containerID := m.Items[m.Cursor].Container.ID

	return func() tea.Msg {
		timeout := 10
		_, err := m.DockerClient.ContainerStop(context.Background(), containerID, client.ContainerStopOptions{Timeout: &timeout})
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed to stop: %v", err), Success: false}
		}
		return models.ActionResultMsg{Message: "Container stopped", Success: true}
	}
}

func RestartContainer(m *models.Model) tea.Cmd {
	if m.Cursor >= len(m.Items) || !m.Items[m.Cursor].IsContainer {
		return nil
	}

	containerID := m.Items[m.Cursor].Container.ID

	return func() tea.Msg {
		timeout := 10
		_, err := m.DockerClient.ContainerRestart(context.Background(), containerID, client.ContainerRestartOptions{Timeout: &timeout})
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed to restart: %v", err), Success: false}
		}
		return models.ActionResultMsg{Message: "Container restarted", Success: true}
	}
}

func DeleteContainer(m *models.Model) tea.Cmd {
	if m.Cursor >= len(m.Items) || !m.Items[m.Cursor].IsContainer {
		return nil
	}

	containerID := m.Items[m.Cursor].Container.ID

	return func() tea.Msg {
		_, err := m.DockerClient.ContainerRemove(context.Background(), containerID, client.ContainerRemoveOptions{Force: true})
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed to delete: %v", err), Success: false}
		}
		return models.ActionResultMsg{Message: "Container deleted", Success: true}
	}
}

func LoadLogs(m *models.Model) tea.Cmd {
	if m.Cursor >= len(m.Items) || !m.Items[m.Cursor].IsContainer {
		return nil
	}

	containerID := m.Items[m.Cursor].Container.ID

	return func() tea.Msg {
		reader, err := m.DockerClient.ContainerLogs(context.Background(), containerID, client.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Tail:       "100",
			Timestamps: true,
		})
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed to load logs: %v", err), Success: false}
		}
		defer reader.Close()

		var lines []string
		buf := make([]byte, 8) // Docker log header is 8 bytes

		for {
			// Read header (8 bytes: stream type, padding, size)
			_, err := io.ReadFull(reader, buf)
			if err == io.EOF {
				break
			}
			if err != nil {
				break
			}

			// Extract payload size from header (last 4 bytes, big-endian)
			size := uint32(buf[4])<<24 | uint32(buf[5])<<16 | uint32(buf[6])<<8 | uint32(buf[7])

			// Read the actual log line
			payload := make([]byte, size)
			_, err = io.ReadFull(reader, payload)
			if err != nil {
				break
			}

			line := strings.TrimSpace(string(payload))
			if line != "" {
				lines = append(lines, line)
			}
		}

		return models.LogsLoadedMsg{Lines: lines}
	}
}

func ExecShell(m *models.Model) tea.Cmd {
	if m.Cursor >= len(m.Items) || !m.Items[m.Cursor].IsContainer {
		return nil
	}

	containerID := m.Items[m.Cursor].Container.ID
	containerName := m.Items[m.Cursor].Container.Name

	// Try different shells in order of preference
	shells := []string{"/bin/bash", "/bin/sh", "/bin/ash"}

	var selectedShell string
	for _, shell := range shells {
		// Test if shell exists
		testCmd := exec.Command("docker", "exec", containerID, "test", "-f", shell)
		if err := testCmd.Run(); err == nil {
			selectedShell = shell
			break
		}
	}

	if selectedShell == "" {
		selectedShell = "/bin/sh" // Fallback
	}

	cmd := exec.Command("docker", "exec", "-it", containerID, selectedShell)

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return models.ActionResultMsg{
				Message: fmt.Sprintf("Failed to exec into %s: %v", containerName, err),
				Success: false,
			}
		}
		return models.ActionResultMsg{
			Message: fmt.Sprintf("Exited shell in %s", containerName),
			Success: true,
		}
	})
}

func OpenPortInBrowser(m *models.Model) tea.Cmd {
	if m.Cursor >= len(m.Items) || !m.Items[m.Cursor].IsContainer {
		return func() tea.Msg {
			return models.ActionResultMsg{Message: "No container selected", Success: false}
		}
	}

	c := m.Items[m.Cursor].Container
	if len(c.Ports) == 0 {
		return func() tea.Msg {
			return models.ActionResultMsg{Message: "No ports available", Success: false}
		}
	}

	if m.SelectedPort >= len(c.Ports) {
		return func() tea.Msg {
			return models.ActionResultMsg{Message: "Invalid port selection", Success: false}
		}
	}

	port := c.Ports[m.SelectedPort]
	if port.PublicPort == 0 {
		return func() tea.Msg {
			return models.ActionResultMsg{Message: "Port is not published", Success: false}
		}
	}

	url := fmt.Sprintf("http://localhost:%d", port.PublicPort)

	return func() tea.Msg {
		var cmd *exec.Cmd

		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		default: // linux and others
			cmd = exec.Command("xdg-open", url)
		}

		// Start the command without waiting for it to complete
		if err := cmd.Start(); err != nil {
			return models.ActionResultMsg{
				Message: fmt.Sprintf("Failed to open browser: %v", err),
				Success: false,
			}
		}

		return models.ActionResultMsg{
			Message: fmt.Sprintf("Opened %s in browser", url),
			Success: true,
		}
	}
}

func InitialModel() (models.Model, error) {
	// Connect to Docker
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return models.Model{}, err
	}

	m := models.Model{
		DockerClient: cli,
		ViewMode:     models.ViewDetails,
		NavMode:      models.NavContainers,
	}

	// Load initial data
	if err := LoadContainers(&m); err != nil {
		return models.Model{}, err
	}
	if err := LoadVolumes(&m); err != nil {
		return models.Model{}, err
	}
	if err := LoadImages(&m); err != nil {
		return models.Model{}, err
	}
	if err := LoadNetworks(&m); err != nil {
		return models.Model{}, err
	}

	return m, nil
}

func Init(m models.Model) tea.Cmd {
	return nil
}

func Quit(m *models.Model) {
	if m.DockerClient != nil {
		m.DockerClient.Close()
	}
}

func LoadStats(m *models.Model) tea.Cmd {
	if m.Cursor >= len(m.Items) || !m.Items[m.Cursor].IsContainer {
		return nil
	}

	containerID := m.Items[m.Cursor].Container.ID

	return func() tea.Msg {
		ctx := context.Background()

		// Get stats with stream=false to get a single snapshot
		stats, err := m.DockerClient.ContainerStats(ctx, containerID, client.ContainerStatsOptions{Stream: false})
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed to load stats: %v", err), Success: false}
		}
		defer stats.Body.Close()

		// Decode the stats
		var v container.StatsResponse
		if err := json.NewDecoder(stats.Body).Decode(&v); err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed to decode stats: %v", err), Success: false}
		}

		// Calculate CPU percentage
		cpuDelta := float64(v.CPUStats.CPUUsage.TotalUsage) - float64(v.PreCPUStats.CPUUsage.TotalUsage)
		systemDelta := float64(v.CPUStats.SystemUsage) - float64(v.PreCPUStats.SystemUsage)
		cpuPercent := 0.0
		if systemDelta > 0.0 && cpuDelta > 0.0 {
			cpuPercent = (cpuDelta / systemDelta) * float64(len(v.CPUStats.CPUUsage.PercpuUsage)) * 100.0
		}

		// Calculate memory usage
		memUsage := v.MemoryStats.Usage
		memLimit := v.MemoryStats.Limit
		memPercent := 0.0
		if memLimit > 0 {
			memPercent = (float64(memUsage) / float64(memLimit)) * 100.0
		}

		// Format memory usage
		memUsageStr := formatBytes(memUsage)
		memLimitStr := formatBytes(memLimit)

		// Calculate network I/O
		var rxBytes, txBytes uint64
		for _, network := range v.Networks {
			rxBytes += network.RxBytes
			txBytes += network.TxBytes
		}
		netIO := fmt.Sprintf("%s / %s", formatBytes(rxBytes), formatBytes(txBytes))

		// Calculate block I/O
		var readBytes, writeBytes uint64
		for _, bio := range v.BlkioStats.IoServiceBytesRecursive {
			if bio.Op == "read" || bio.Op == "Read" {
				readBytes += bio.Value
			} else if bio.Op == "write" || bio.Op == "Write" {
				writeBytes += bio.Value
			}
		}
		blockIO := fmt.Sprintf("%s / %s", formatBytes(readBytes), formatBytes(writeBytes))

		// Get PIDs
		pids := fmt.Sprintf("%d", v.PidsStats.Current)

		return models.StatsLoadedMsg{
			Stats: &models.ContainerStats{
				CPUPerc:     fmt.Sprintf("%.2f%%", cpuPercent),
				MemUsage:    fmt.Sprintf("%s / %s", memUsageStr, memLimitStr),
				MemPerc:     fmt.Sprintf("%.2f%%", memPercent),
				NetIO:       netIO,
				BlockIO:     blockIO,
				PIDs:        pids,
				ContainerID: containerID,
			},
		}
	}
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func DeleteVolume(m *models.Model) tea.Cmd {
	if m.Cursor >= len(m.Items) || !m.Items[m.Cursor].IsVolume {
		return nil
	}

	volumeName := m.Items[m.Cursor].Volume.Name

	return func() tea.Msg {
		_, err := m.DockerClient.VolumeRemove(context.Background(), volumeName, client.VolumeRemoveOptions{Force: true})
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed to delete: %v", err), Success: false}
		}

		// Reload volumes
		volumeList, err := m.DockerClient.VolumeList(context.Background(), client.VolumeListOptions{})
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed to reload volumes: %v", err), Success: false}
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

		return models.VolumesLoadedMsg{Volumes: volumes}
	}
}

func DeleteImage(m *models.Model) tea.Cmd {
	if m.Cursor >= len(m.Items) || !m.Items[m.Cursor].IsImage {
		return nil
	}

	imageID := m.Items[m.Cursor].Image.ID

	return func() tea.Msg {
		_, err := m.DockerClient.ImageRemove(context.Background(), imageID, client.ImageRemoveOptions{Force: true})
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed to delete: %v", err), Success: false}
		}

		// Reload images
		imageList, err := m.DockerClient.ImageList(context.Background(), client.ImageListOptions{All: true})
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed to reload images: %v", err), Success: false}
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

		return models.ImagesLoadedMsg{Images: images}
	}
}

func LoadInspect(m *models.Model) tea.Cmd {
	if m.Cursor >= len(m.Items) || !m.Items[m.Cursor].IsContainer {
		return nil
	}

	containerID := m.Items[m.Cursor].Container.ID

	return func() tea.Msg {
		data, err := m.DockerClient.ContainerInspect(context.Background(), containerID, client.ContainerInspectOptions{})
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed to inspect: %v", err), Success: false}
		}

		// Convert to pretty JSON
		jsonData, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed to marshal JSON: %v", err), Success: false}
		}

		return models.InspectLoadedMsg{Data: string(jsonData)}
	}
}
