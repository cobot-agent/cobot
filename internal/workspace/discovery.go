package workspace

func Discover(startDir string) (*Workspace, error) {
	m, err := NewManager()
	if err != nil {
		return nil, err
	}
	return m.Discover(startDir)
}

func DiscoverOrDefault(startDir string) (*Workspace, error) {
	m, err := NewManager()
	if err != nil {
		return nil, err
	}
	return m.ResolveByNameOrDiscover("", startDir)
}
