package app

import (
	"sort"

	"gdocker/models"
)

func groupByProject(containers []models.Container) ([]models.Container, []models.ComposeGroup) {
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

func rebuildContainerItems(m *Model) {
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

func rebuildVolumeItems(m *Model) {
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

func rebuildImageItems(m *Model) {
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

func rebuildNetworkItems(m *Model) {
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
