package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"gdocker/models"
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

// Client talks to one Docker daemon.
type Client struct {
	cli  *client.Client
	host string // as configured, "" for the local default
}

// Connect builds a client for host ("" for DOCKER_HOST or the local
// daemon, ssh://... through the docker CLI's connection helper).
func Connect(host string) (*Client, error) {
	cli, err := buildDockerClient(host)
	if err != nil {
		return nil, err
	}
	return &Client{cli: cli, host: host}, nil
}

// Host is the host the client was connected to, "" for the default.
func (c *Client) Host() string { return c.host }

// Close stops a log follower and closes the connection.
func (c *Client) Close() {
	c.StopFollowLogs()
	c.cli.Close()
}

// RefreshContainers lists the containers again; prev are the ones shown, to
// keep their loaded environments and notice recreated containers.
func (c *Client) RefreshContainers(prev []models.Container) tea.Cmd {
	cli := c.cli
	prevIDsByName := make(map[string]string, len(prev))
	for _, c := range prev {
		id := containerRuntimeID(&c)
		if id == "" {
			continue
		}
		prevIDsByName[c.Name] = id
	}
	envCache := envCacheFromContainers(prev)

	return func() tea.Msg {
		containerList, err := cli.ContainerList(context.Background(), client.ContainerListOptions{All: true})
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
func (c *Client) RefreshVolumes() tea.Cmd {
	cli := c.cli
	return func() tea.Msg {
		volumes, err := listVolumes(cli)
		if err != nil {
			return nil
		}
		return models.VolumesLoadedMsg{Volumes: volumes}
	}
}

func (c *Client) RefreshImages() tea.Cmd {
	cli := c.cli
	return func() tea.Msg {
		images, err := listImages(cli)
		if err != nil {
			return nil
		}
		return models.ImagesLoadedMsg{Images: images}
	}
}

func (c *Client) RefreshNetworks() tea.Cmd {
	cli := c.cli
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

// StopFollowLogs ends the log follower, if one runs.
func (c *Client) StopFollowLogs() {
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

func (c *Client) Start(ctr *models.Container) tea.Cmd {
	if ctr == nil {
		return nil
	}
	cli := c.cli

	containerID := containerRuntimeID(ctr)

	return func() tea.Msg {
		_, err := cli.ContainerStart(context.Background(), containerID, client.ContainerStartOptions{})
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed to start: %v", err), Success: false}
		}
		return models.ActionResultMsg{Message: "Container started", Success: true}
	}
}

func (c *Client) Stop(ctr *models.Container) tea.Cmd {
	if ctr == nil {
		return nil
	}
	cli := c.cli

	containerID := containerRuntimeID(ctr)

	return func() tea.Msg {
		timeout := 10
		_, err := cli.ContainerStop(context.Background(), containerID, client.ContainerStopOptions{Timeout: &timeout})
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed to stop: %v", err), Success: false}
		}
		return models.ActionResultMsg{Message: "Container stopped", Success: true}
	}
}

func (c *Client) Restart(ctr *models.Container) tea.Cmd {
	if ctr == nil {
		return nil
	}
	cli := c.cli

	containerID := containerRuntimeID(ctr)

	return func() tea.Msg {
		timeout := 10
		_, err := cli.ContainerRestart(context.Background(), containerID, client.ContainerRestartOptions{Timeout: &timeout})
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed to restart: %v", err), Success: false}
		}
		return models.ActionResultMsg{Message: "Container restarted", Success: true}
	}
}

func (c *Client) Delete(ctr *models.Container) tea.Cmd {
	if ctr == nil {
		return nil
	}
	cli := c.cli

	containerID := containerRuntimeID(ctr)

	return func() tea.Msg {
		_, err := cli.ContainerRemove(context.Background(), containerID, client.ContainerRemoveOptions{Force: true})
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed to delete: %v", err), Success: false}
		}
		return models.ActionResultMsg{Message: "Container deleted", Success: true}
	}
}

// LoadLogs fetches the last tail lines of a container's log.
func (c *Client) LoadLogs(ctr *models.Container, tail int) tea.Cmd {
	if ctr == nil {
		return nil
	}
	cli := c.cli
	containerID := containerRuntimeID(ctr)
	tailLines := tail
	if tailLines < 1 {
		tailLines = 1000
	}

	return func() tea.Msg {
		reader, err := cli.ContainerLogs(context.Background(), containerID, client.ContainerLogsOptions{
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

// FollowLogs waits for the next lines of a container's log from since on,
// starting the follower if needed.
func (c *Client) FollowLogs(ctr *models.Container, since time.Time) tea.Cmd {
	if ctr == nil {
		c.StopFollowLogs()
		return nil
	}
	containerID := containerRuntimeID(ctr)
	if since.IsZero() {
		since = time.Now()
	}
	follower := ensureLogFollower(c.cli, containerID, since)
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

// ExecShell opens a shell in the container, in the terminal.
func (c *Client) ExecShell(ctr *models.Container) tea.Cmd {
	if ctr == nil {
		return nil
	}
	containerID, containerName := containerRuntimeID(ctr), ctr.Name

	// Let the container pick its best shell; probing from here would block
	// the UI with one docker call per candidate.
	const pickShell = "command -v bash >/dev/null 2>&1 && exec bash || exec sh"
	cmd := exec.Command("docker", "exec", "-it", containerID, "sh", "-c", pickShell)
	if c.host != "" {
		// Target the host gdocker is connected to, not the CLI default.
		cmd.Env = append(os.Environ(), "DOCKER_HOST="+c.host)
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

// OpenPort opens the container's i-th published port in the browser.
func (*Client) OpenPort(c *models.Container, i int) tea.Cmd {
	if c == nil {
		return func() tea.Msg {
			return models.ActionResultMsg{Message: "No container selected", Success: false}
		}
	}

	if len(c.Ports) == 0 {
		return func() tea.Msg {
			return models.ActionResultMsg{Message: "No ports available", Success: false}
		}
	}

	if i >= len(c.Ports) {
		return func() tea.Msg {
			return models.ActionResultMsg{Message: "Invalid port selection", Success: false}
		}
	}

	port := c.Ports[i]
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

// ConnectedMsg reports a new connection and what it shows.
type ConnectedMsg struct {
	Client *Client
	Host   string
	Snapshot
}

// Snapshot is everything the lists show.
type Snapshot struct {
	Containers []models.Container
	Volumes    []models.Volume
	Images     []models.Image
	Networks   []models.Network
}

// Load lists containers, volumes, images and networks.
func (c *Client) Load() (Snapshot, error) {
	var s Snapshot
	var err error
	if s.Containers, err = c.listContainers(nil); err != nil {
		return s, fmt.Errorf("loading containers: %w", err)
	}
	if s.Volumes, err = listVolumes(c.cli); err != nil {
		return s, fmt.Errorf("loading volumes: %w", err)
	}
	if s.Images, err = listImages(c.cli); err != nil {
		return s, fmt.Errorf("loading images: %w", err)
	}
	if s.Networks, err = listNetworks(c.cli); err != nil {
		return s, fmt.Errorf("loading networks: %w", err)
	}
	return s, nil
}

// Connect switches to another host: the result is a ConnectedMsg, or an
// ActionResultMsg saying why it failed.
func (*Client) Connect(host string) tea.Cmd {
	return func() tea.Msg {
		c, err := Connect(host)
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Connect failed: %v", err), Success: false}
		}
		if _, err := c.cli.Ping(context.Background(), client.PingOptions{}); err != nil {
			c.cli.Close()
			return models.ActionResultMsg{Message: fmt.Sprintf("Ping failed: %v", err), Success: false}
		}
		snap, err := c.Load()
		if err != nil {
			c.cli.Close()
			return models.ActionResultMsg{Message: "Failed " + err.Error(), Success: false}
		}
		return ConnectedMsg{Client: c, Host: host, Snapshot: snap}
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

// LoadStats takes a stats sample of a container, tagged with seq.
func (c *Client) LoadStats(ctr *models.Container, seq int) tea.Cmd {
	if ctr == nil {
		return nil
	}
	cli := c.cli
	containerID := containerRuntimeID(ctr)

	return func() tea.Msg {
		ctx := context.Background()

		// Take a first snapshot.
		v, err := getContainerStatsSnapshot(ctx, cli, containerID)
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed to load stats: %v", err), Success: false}
		}

		// Calculate CPU from Docker's pre-cpu delta first.
		cpuPercent := calculateCPUPercent(v.CPUStats, v.PreCPUStats)

		// Some daemons/cgroup setups return empty/zero pre-cpu on first read.
		// Fallback: take a second snapshot and compute deltas between snapshots.
		if cpuPercent == 0.0 {
			time.Sleep(250 * time.Millisecond)
			v2, err := getContainerStatsSnapshot(ctx, cli, containerID)
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

func (c *Client) DeleteVolume(v *models.Volume) tea.Cmd {
	if v == nil {
		return nil
	}
	cli := c.cli
	volumeName := v.Name

	return func() tea.Msg {
		_, err := cli.VolumeRemove(context.Background(), volumeName, client.VolumeRemoveOptions{Force: true})
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed to delete: %v", err), Success: false}
		}

		volumes, err := listVolumes(cli)
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed to reload volumes: %v", err), Success: false}
		}
		return models.VolumesLoadedMsg{Volumes: volumes, Status: "Volume " + volumeName + " deleted"}
	}
}

func (c *Client) DeleteImage(img *models.Image) tea.Cmd {
	if img == nil {
		return nil
	}
	cli := c.cli
	imageID := img.ID

	return func() tea.Msg {
		_, err := cli.ImageRemove(context.Background(), imageID, client.ImageRemoveOptions{Force: true})
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed to delete: %v", err), Success: false}
		}

		images, err := listImages(cli)
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed to reload images: %v", err), Success: false}
		}
		return models.ImagesLoadedMsg{Images: images, Status: "Image " + imageID + " deleted"}
	}
}

func (c *Client) LoadEnv(ctr *models.Container) tea.Cmd {
	if ctr == nil {
		return nil
	}
	cli := c.cli

	containerID := containerRuntimeID(ctr)
	return func() tea.Msg {
		data, err := cli.ContainerInspect(context.Background(), containerID, client.ContainerInspectOptions{})
		if err != nil {
			return models.ActionResultMsg{Message: fmt.Sprintf("Failed to load env: %v", err), Success: false}
		}
		return models.EnvLoadedMsg{
			ContainerID: containerID,
			Env:         data.Container.Config.Env,
		}
	}
}

func (c *Client) LoadInspect(ctr *models.Container) tea.Cmd {
	if ctr == nil {
		return nil
	}
	cli := c.cli

	containerID := containerRuntimeID(ctr)

	return func() tea.Msg {
		data, err := cli.ContainerInspect(context.Background(), containerID, client.ContainerInspectOptions{})
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
