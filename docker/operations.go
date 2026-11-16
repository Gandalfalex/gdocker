package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"gdocker/config"
	"gdocker/models"
	"gdocker/ui"
	"gdocker/ui/logviewer"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/docker/cli/cli/connhelper"
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
	models.RefreshVolumesFunc = RefreshVolumes
	models.RefreshImagesFunc = RefreshImages
	models.RefreshNetworksFunc = RefreshNetworks
	models.LoadEnvFunc = LoadEnv
	models.ConnectDockerHostFunc = ConnectDockerHost
	models.StartContainerFunc = StartContainer
	models.StopContainerFunc = StopContainer
	models.RestartContainerFunc = RestartContainer
	models.DeleteContainerFunc = DeleteContainer
	models.DeleteVolumeFunc = DeleteVolume
	models.DeleteImageFunc = DeleteImage
	models.LoadLogsFunc = LoadLogs
	models.FollowLogsFunc = FollowLogs
	models.StopFollowLogsFunc = StopFollowLogs
	models.LoadInspectFunc = LoadInspect
	models.ExecShellFunc = ExecShell
	models.OpenPortInBrowserFunc = OpenPortInBrowser
	models.QuitFunc = Quit
	models.LoadStatsFunc = LoadStats
}

func RefreshContainers(m *models.Model) tea.Cmd {
	prevIDsByName := make(map[string]string, len(m.Containers))
	for _, c := range m.Containers {
		id := containerRuntimeID(&c)
		if id == "" {
			continue
		}
		prevIDsByName[c.Name] = id
	}
	envCache := envCacheFromContainers(m.Containers)

	return func() tea.Msg {
		containerList, err := m.DockerClient.ContainerList(context.Background(), client.ContainerListOptions{All: true})
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Error: %v", err), Success: false}
		}
		containers := containersFromList(containerList.Items, envCache)

		recreated := make([]models.ContainerRecreated, 0)
		for _, c := range containers {
			oldID, ok := prevIDsByName[c.Name]
			if !ok {
				continue
			}
			newID := containerRuntimeID(&c)
			if oldID != "" && newID != "" && oldID != newID {
				recreated = append(recreated, models.ContainerRecreated{
					Name:  c.Name,
					OldID: oldID,
					NewID: newID,
				})
			}
		}

		return models.ContainersRefreshedMsg{
			Containers: containers,
			Recreated:  recreated,
		}
	}
}

// RefreshVolumes, RefreshImages and RefreshNetworks reload a resource list in
// the background. Errors are dropped: the next refresh will retry.
func RefreshVolumes(m *models.Model) tea.Cmd {
	cli := m.DockerClient
	return func() tea.Msg {
		volumes, err := listVolumes(cli)
		if err != nil {
			return nil
		}
		return models.VolumesLoadedMsg{Volumes: volumes}
	}
}

func RefreshImages(m *models.Model) tea.Cmd {
	cli := m.DockerClient
	return func() tea.Msg {
		images, err := listImages(cli)
		if err != nil {
			return nil
		}
		return models.ImagesLoadedMsg{Images: images}
	}
}

func RefreshNetworks(m *models.Model) tea.Cmd {
	cli := m.DockerClient
	return func() tea.Msg {
		networks, err := listNetworks(cli)
		if err != nil {
			return nil
		}
		return models.NetworksLoadedMsg{Networks: networks}
	}
}

type logFollowState struct {
	containerID string
	ch          chan tea.Msg
	cancel      context.CancelFunc
}

var (
	logFollowMu    sync.Mutex
	activeFollower *logFollowState
	logFollowRun   = runLogFollower
)

func ensureLogFollower(cli *client.Client, containerID string, since time.Time) *logFollowState {
	logFollowMu.Lock()
	defer logFollowMu.Unlock()

	if activeFollower != nil && activeFollower.containerID == containerID {
		return activeFollower
	}

	stopActiveLogFollowerLocked()

	ctx, cancel := context.WithCancel(context.Background())
	state := &logFollowState{
		containerID: containerID,
		ch:          make(chan tea.Msg, 64),
		cancel:      cancel,
	}
	activeFollower = state
	go logFollowRun(ctx, cli, containerID, since, state.ch)
	return state
}

func StopFollowLogs(_ *models.Model) {
	logFollowMu.Lock()
	defer logFollowMu.Unlock()
	stopActiveLogFollowerLocked()
}

func stopActiveLogFollowerLocked() {
	if activeFollower == nil {
		return
	}
	activeFollower.cancel()
	activeFollower = nil
}

func runLogFollower(ctx context.Context, cli *client.Client, containerID string, since time.Time, ch chan tea.Msg) {
	defer close(ch)

	reader, err := cli.ContainerLogs(ctx, containerID, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "0",
		Follow:     true,
		Timestamps: true,
		Since:      since.Format(time.RFC3339Nano),
	})
	if err != nil {
		select {
		case ch <- models.ActionResultMsg{Message: fmt.Sprintf("Failed to follow logs: %v", err), Success: false}:
		case <-ctx.Done():
		}
		return
	}
	defer reader.Close()

	buf := make([]byte, 8) // Docker log header is 8 bytes
	for {
		_, err := io.ReadFull(reader, buf)
		if err != nil {
			return
		}

		size := uint32(buf[4])<<24 | uint32(buf[5])<<16 | uint32(buf[6])<<8 | uint32(buf[7])
		payload := make([]byte, size)
		_, err = io.ReadFull(reader, payload)
		if err != nil {
			return
		}

		line := strings.TrimSpace(string(payload))
		if line == "" {
			continue
		}

		newest := since
		if ts, ok := parseLogTimestamp(line); ok {
			newest = ts
			since = ts
		}

		select {
		case ch <- models.LogsFollowedMsg{Lines: []string{line}, Since: newest}:
		case <-ctx.Done():
			return
		}
	}
}

func StartContainer(m *models.Model) tea.Cmd {
	if m.Cursor >= len(m.Items) || !m.Items[m.Cursor].IsContainer {
		return nil
	}

	containerID := containerRuntimeID(m.Items[m.Cursor].Container)

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

	containerID := containerRuntimeID(m.Items[m.Cursor].Container)

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

	containerID := containerRuntimeID(m.Items[m.Cursor].Container)

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

	containerID := containerRuntimeID(m.Items[m.Cursor].Container)

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

	containerID := containerRuntimeID(m.Items[m.Cursor].Container)
	tailLines := m.LogTailLines
	if tailLines < 1 {
		tailLines = 1000
	}

	return func() tea.Msg {
		reader, err := m.DockerClient.ContainerLogs(context.Background(), containerID, client.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Tail:       strconv.Itoa(tailLines),
			Timestamps: true,
		})
		if err != nil {
			return models.LogsFailedMsg{Err: err}
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

		since := time.Now()
		if len(lines) > 0 {
			if ts, ok := parseLogTimestamp(lines[len(lines)-1]); ok {
				since = ts
			}
		}

		return models.LogsLoadedMsg{
			Lines:  lines,
			Follow: true,
			Since:  since,
		}
	}
}

func FollowLogs(m *models.Model) tea.Cmd {
	if m.ViewMode != models.ViewLogs || !m.LogViewer.FollowingLogs {
		StopFollowLogs(m)
		return nil
	}
	if m.Cursor >= len(m.Items) || !m.Items[m.Cursor].IsContainer {
		StopFollowLogs(m)
		return nil
	}

	containerID := containerRuntimeID(m.Items[m.Cursor].Container)
	since := m.LogSince
	if since.IsZero() {
		since = time.Now()
	}
	follower := ensureLogFollower(m.DockerClient, containerID, since)
	if follower == nil {
		return nil
	}
	return func() tea.Msg {
		msg, ok := <-follower.ch
		if !ok {
			return models.LogFollowStoppedMsg{}
		}
		return msg
	}
}

func decodeDockerLogStream(reader io.Reader, fallbackSince time.Time) ([]string, time.Time) {
	var lines []string
	buf := make([]byte, 8) // Docker log header is 8 bytes
	newest := fallbackSince

	for {
		_, err := io.ReadFull(reader, buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		size := uint32(buf[4])<<24 | uint32(buf[5])<<16 | uint32(buf[6])<<8 | uint32(buf[7])
		payload := make([]byte, size)
		_, err = io.ReadFull(reader, payload)
		if err != nil {
			break
		}

		line := strings.TrimSpace(string(payload))
		if line == "" {
			continue
		}
		lines = append(lines, line)
		if ts, ok := parseLogTimestamp(line); ok && ts.After(newest) {
			newest = ts
		}
	}

	return lines, newest
}

func parseLogTimestamp(line string) (time.Time, bool) {
	parts := strings.SplitN(line, " ", 2)
	if len(parts) < 1 {
		return time.Time{}, false
	}
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, false
	}
	return ts, true
}

func containerRuntimeID(c *models.Container) string {
	if c == nil {
		return ""
	}
	if c.FullID != "" {
		return c.FullID
	}
	return c.ID
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func ExecShell(m *models.Model) tea.Cmd {
	if m.Cursor >= len(m.Items) || !m.Items[m.Cursor].IsContainer {
		return nil
	}

	containerID := containerRuntimeID(m.Items[m.Cursor].Container)
	containerName := m.Items[m.Cursor].Container.Name

	// Let the container pick its best shell; probing from here would block
	// the UI with one docker call per candidate.
	const pickShell = "command -v bash >/dev/null 2>&1 && exec bash || exec sh"
	cmd := exec.Command("docker", "exec", "-it", containerID, "sh", "-c", pickShell)
	if m.DockerHost != "" {
		// Target the host gdocker is connected to, not the CLI default.
		cmd.Env = append(os.Environ(), "DOCKER_HOST="+m.DockerHost)
	}

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return models.ActionResultMsg{
				Message: fmt.Sprintf("Shell in %s ended with: %v", containerName, err),
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
	// Load app config
	appConfig, configErr := config.Load()
	if configErr != nil {
		// If config loading fails, use defaults (and say so below).
		appConfig = config.DefaultAppConfig()
	}

	// Connect to Docker. Configured host overrides DOCKER_HOST.
	cli, err := buildDockerClient(appConfig.Docker.Host)
	if err != nil {
		return models.Model{}, err
	}

	m := models.Model{
		KeyBindings:     &appConfig.KeyBindings,
		UIConfig:        &appConfig.UI,
		DockerClient:    cli,
		DockerHost:      appConfig.Docker.Host,
		HostAliases:     appConfig.Docker.HostAliases,
		ViewMode:        models.ViewDetails,
		NavMode:         models.NavContainers,
		AutoRefreshSecs: appConfig.Docker.AutoRefreshSeconds,
		LogTailLines:    appConfig.Docker.LogTailLines,
		LogViewer:       logviewer.New(&appConfig.KeyBindings.Logs, appConfig.UI.ShowLineNumbers, appConfig.UI.WrapLogs, ui.CopyToClipboard),
	}

	if configErr != nil {
		m.StatusMessage = fmt.Sprintf("Config ignored, using defaults: %v", configErr)
		m.StatusError = true
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
	StopFollowLogs(m)
	if m.DockerClient != nil {
		m.DockerClient.Close()
	}
}

func ConnectDockerHost(m *models.Model, host string) tea.Cmd {
	return func() tea.Msg {
		cli, err := buildDockerClient(host)
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Connect failed: %v", err), Success: false}
		}

		if _, err := cli.Ping(context.Background(), client.PingOptions{}); err != nil {
			cli.Close()
			return models.ActionResultMsg{Message: fmt.Sprintf("Ping failed: %v", err), Success: false}
		}

		tmp := models.Model{DockerClient: cli}
		if err := LoadContainers(&tmp); err != nil {
			cli.Close()
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed loading containers: %v", err), Success: false}
		}
		if err := LoadVolumes(&tmp); err != nil {
			cli.Close()
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed loading volumes: %v", err), Success: false}
		}
		if err := LoadImages(&tmp); err != nil {
			cli.Close()
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed loading images: %v", err), Success: false}
		}
		if err := LoadNetworks(&tmp); err != nil {
			cli.Close()
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed loading networks: %v", err), Success: false}
		}

		return models.DockerConnectedMsg{
			Client:     cli,
			Host:       host,
			Containers: tmp.Containers,
			Volumes:    tmp.Volumes,
			Images:     tmp.Images,
			Networks:   tmp.Networks,
		}
	}
}

func buildDockerClient(host string) (*client.Client, error) {
	if strings.HasPrefix(host, "ssh://") {
		helper, err := connhelper.GetConnectionHelper(host)
		if err != nil {
			return nil, err
		}
		if helper != nil {
			httpClient := &http.Client{
				Transport: &http.Transport{
					DialContext: helper.Dialer,
				},
			}
			return client.NewClientWithOpts(
				client.WithHost(helper.Host),
				client.WithAPIVersionNegotiation(),
				client.WithHTTPClient(httpClient),
			)
		}
	}

	opts := []client.Opt{
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	}
	if host != "" {
		opts = append(opts, client.WithHost(host))
	}
	return client.NewClientWithOpts(opts...)
}

func LoadStats(m *models.Model) tea.Cmd {
	if m.Cursor >= len(m.Items) || !m.Items[m.Cursor].IsContainer {
		return nil
	}

	containerID := containerRuntimeID(m.Items[m.Cursor].Container)
	seq := m.StatsSeq

	return func() tea.Msg {
		ctx := context.Background()

		// Take a first snapshot.
		v, err := getContainerStatsSnapshot(ctx, m.DockerClient, containerID)
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed to load stats: %v", err), Success: false}
		}

		// Calculate CPU from Docker's pre-cpu delta first.
		cpuPercent := calculateCPUPercent(v.CPUStats, v.PreCPUStats)

		// Some daemons/cgroup setups return empty/zero pre-cpu on first read.
		// Fallback: take a second snapshot and compute deltas between snapshots.
		if cpuPercent == 0.0 {
			time.Sleep(250 * time.Millisecond)
			v2, err := getContainerStatsSnapshot(ctx, m.DockerClient, containerID)
			if err == nil {
				cpuPercent = calculateCPUPercent(v2.CPUStats, v.CPUStats)
				v = v2 // Keep the freshest values for memory/net/block/pids.
			}
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
			switch bio.Op {
			case "read", "Read":
				readBytes += bio.Value
			case "write", "Write":
				writeBytes += bio.Value
			}
		}
		blockIO := fmt.Sprintf("%s / %s", formatBytes(readBytes), formatBytes(writeBytes))

		// Get PIDs
		pids := fmt.Sprintf("%d", v.PidsStats.Current)

		return models.StatsLoadedMsg{
			Seq: seq,
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

func getContainerStatsSnapshot(ctx context.Context, cli *client.Client, containerID string) (container.StatsResponse, error) {
	stats, err := cli.ContainerStats(ctx, containerID, client.ContainerStatsOptions{Stream: false})
	if err != nil {
		return container.StatsResponse{}, err
	}
	defer stats.Body.Close()

	var v container.StatsResponse
	if err := json.NewDecoder(stats.Body).Decode(&v); err != nil {
		return container.StatsResponse{}, err
	}
	return v, nil
}

func calculateCPUPercent(current, previous container.CPUStats) float64 {
	cpuDelta := float64(current.CPUUsage.TotalUsage) - float64(previous.CPUUsage.TotalUsage)
	systemDelta := float64(current.SystemUsage) - float64(previous.SystemUsage)
	if systemDelta <= 0.0 || cpuDelta <= 0.0 {
		return 0.0
	}

	cpuCount := float64(current.OnlineCPUs)
	if cpuCount == 0.0 {
		cpuCount = float64(len(current.CPUUsage.PercpuUsage))
	}
	if cpuCount == 0.0 {
		cpuCount = 1.0
	}

	return (cpuDelta / systemDelta) * cpuCount * 100.0
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

		volumes, err := listVolumes(m.DockerClient)
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed to reload volumes: %v", err), Success: false}
		}
		return models.VolumesLoadedMsg{Volumes: volumes, Status: "Volume " + volumeName + " deleted"}
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

		images, err := listImages(m.DockerClient)
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed to reload images: %v", err), Success: false}
		}
		return models.ImagesLoadedMsg{Images: images, Status: "Image " + imageID + " deleted"}
	}
}

func LoadEnv(m *models.Model) tea.Cmd {
	if m.Cursor >= len(m.Items) || !m.Items[m.Cursor].IsContainer {
		return nil
	}

	containerID := containerRuntimeID(m.Items[m.Cursor].Container)
	return func() tea.Msg {
		data, err := m.DockerClient.ContainerInspect(context.Background(), containerID, client.ContainerInspectOptions{})
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed to load env: %v", err), Success: false}
		}
		return models.EnvLoadedMsg{
			ContainerID: containerID,
			Env:         data.Container.Config.Env,
		}
	}
}

func LoadInspect(m *models.Model) tea.Cmd {
	if m.Cursor >= len(m.Items) || !m.Items[m.Cursor].IsContainer {
		return nil
	}

	containerID := containerRuntimeID(m.Items[m.Cursor].Container)

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
