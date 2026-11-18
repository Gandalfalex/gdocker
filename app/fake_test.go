package app

import (
	tk "github.com/Gandalfalex/tuikit"
	"time"

	tea "charm.land/bubbletea/v2"

	"gdocker/models"
)

// fakeBackend records what the app asks of Docker; a nil func does
// nothing.
type fakeBackend struct {
	noBackend
	connect    func(host string) tea.Cmd
	delete     func(c *models.Container) tea.Cmd
	loadEnv    func(c *models.Container) tea.Cmd
	follow     func(c *models.Container, since time.Time) tea.Cmd
	openPort   func(c *models.Container, port int) tea.Cmd
	stopFollow func()
}

func (f *fakeBackend) Connect(host string) tea.Cmd {
	if f.connect == nil {
		return nil
	}
	return f.connect(host)
}

func (f *fakeBackend) Delete(c *models.Container) tea.Cmd {
	if f.delete == nil {
		return nil
	}
	return f.delete(c)
}

func (f *fakeBackend) LoadEnv(c *models.Container) tea.Cmd {
	if f.loadEnv == nil {
		return nil
	}
	return f.loadEnv(c)
}

func (f *fakeBackend) FollowLogs(c *models.Container, since time.Time) tea.Cmd {
	if f.follow == nil {
		return nil
	}
	return f.follow(c, since)
}

func (f *fakeBackend) OpenPort(c *models.Container, port int) tea.Cmd {
	if f.openPort == nil {
		return nil
	}
	return f.openPort(c, port)
}

func (f *fakeBackend) StopFollowLogs() {
	if f.stopFollow != nil {
		f.stopFollow()
	}
}

// withCommand is m with the ":" line open and text typed in it.
func withCommand(m Model, text string) Model {
	m.ensure()
	m.cmd.Open(&m)
	m.prompt.Open(1, tk.ModeCommand, ":", "", text)
	return m
}
