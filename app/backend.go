package app

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"gdocker/docker"
	"gdocker/models"
)

// Backend is the Docker daemon as the app uses it; *docker.Client is the
// real one. Every call returns at once: the work runs in the command.
type Backend interface {
	Host() string
	Connect(host string) tea.Cmd // docker.ConnectedMsg or an ActionResultMsg
	Close()

	RefreshContainers(prev []models.Container) tea.Cmd
	RefreshVolumes() tea.Cmd
	RefreshImages() tea.Cmd
	RefreshNetworks() tea.Cmd

	Start(c *models.Container) tea.Cmd
	Stop(c *models.Container) tea.Cmd
	Restart(c *models.Container) tea.Cmd
	Delete(c *models.Container) tea.Cmd
	DeleteVolume(v *models.Volume) tea.Cmd
	DeleteImage(img *models.Image) tea.Cmd

	LoadLogs(c *models.Container, tail int) tea.Cmd
	FollowLogs(c *models.Container, since time.Time) tea.Cmd
	StopFollowLogs()
	LoadEnv(c *models.Container) tea.Cmd
	LoadInspect(c *models.Container) tea.Cmd
	LoadStats(c *models.Container, seq int) tea.Cmd
	ExecShell(c *models.Container) tea.Cmd
	OpenPort(c *models.Container, port int) tea.Cmd
}

var _ Backend = (*docker.Client)(nil)

// noBackend stands in before a daemon is connected (and in tests): it does
// nothing.
type noBackend struct{}

func (noBackend) Host() string                                    { return "" }
func (noBackend) Connect(string) tea.Cmd                          { return nil }
func (noBackend) Close()                                          {}
func (noBackend) RefreshContainers([]models.Container) tea.Cmd    { return nil }
func (noBackend) RefreshVolumes() tea.Cmd                         { return nil }
func (noBackend) RefreshImages() tea.Cmd                          { return nil }
func (noBackend) RefreshNetworks() tea.Cmd                        { return nil }
func (noBackend) Start(*models.Container) tea.Cmd                 { return nil }
func (noBackend) Stop(*models.Container) tea.Cmd                  { return nil }
func (noBackend) Restart(*models.Container) tea.Cmd               { return nil }
func (noBackend) Delete(*models.Container) tea.Cmd                { return nil }
func (noBackend) DeleteVolume(*models.Volume) tea.Cmd             { return nil }
func (noBackend) DeleteImage(*models.Image) tea.Cmd               { return nil }
func (noBackend) LoadLogs(*models.Container, int) tea.Cmd         { return nil }
func (noBackend) FollowLogs(*models.Container, time.Time) tea.Cmd { return nil }
func (noBackend) StopFollowLogs()                                 {}
func (noBackend) LoadEnv(*models.Container) tea.Cmd               { return nil }
func (noBackend) LoadInspect(*models.Container) tea.Cmd           { return nil }
func (noBackend) LoadStats(*models.Container, int) tea.Cmd        { return nil }
func (noBackend) ExecShell(*models.Container) tea.Cmd             { return nil }
func (noBackend) OpenPort(*models.Container, int) tea.Cmd         { return nil }

// docker is the backend in use, never nil.
func (m *Model) docker() Backend {
	if m.Backend == nil {
		return noBackend{}
	}
	return m.Backend
}
