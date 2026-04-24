package sandbox

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type rewrittenError struct {
	message string
	cause   error
}

func (e *rewrittenError) Error() string { return e.message }

func (e *rewrittenError) Unwrap() error { return e.cause }

func normalizePath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(EvalSymlinks(absPath)), nil
}

// trimPrefixFold removes prefix from s using case-insensitive matching.
// This is needed on Windows where drive letters and path segments may differ in case.
func trimPrefixFold(s, prefix string) string {
	if len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix) {
		return s[len(prefix):]
	}
	return s
}

func pathMatchesRoot(path, root, sep string) bool {
	if path == root || strings.HasPrefix(path, root+sep) {
		return true
	}
	if sep != `\` {
		return false
	}
	pathLower := strings.ToLower(path)
	rootLower := strings.ToLower(root)
	return pathLower == rootLower || strings.HasPrefix(pathLower, rootLower+sep)
}

var shellSegmentReplacer = strings.NewReplacer(
	"\r\n", "\n",
	"&&", "\n",
	"||", "\n",
	"&", "\n",
	";", "\n",
	"|", "\n",
	"$(", "\n",
	"`", "\n",
)

func ShellCommandSegments(cmd string) []string {
	return strings.Split(shellSegmentReplacer.Replace(cmd), "\n")
}

type SandboxConfig struct {
	VirtualRoot     string   `yaml:"virtual_root,omitempty"`
	Root            string   `yaml:"root"`
	AllowPaths      []string `yaml:"allow_paths,omitempty"`
	ReadonlyPaths   []string `yaml:"readonly_paths,omitempty"`
	AllowNetwork    bool     `yaml:"allow_network"`
	BlockedCommands []string `yaml:"blocked_commands,omitempty"`

	allowNetworkSet bool `yaml:"-"`
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	return append([]string(nil), values...)
}

func (s SandboxConfig) Clone() SandboxConfig {
	cloned := s
	cloned.AllowPaths = cloneStrings(s.AllowPaths)
	cloned.ReadonlyPaths = cloneStrings(s.ReadonlyPaths)
	cloned.BlockedCommands = cloneStrings(s.BlockedCommands)
	return cloned
}

func (s *SandboxConfig) SetAllowNetwork(allow bool) {
	if s == nil {
		return
	}
	s.AllowNetwork = allow
	s.allowNetworkSet = true
}

func (s *SandboxConfig) HasAllowNetworkOverride() bool {
	return s != nil && s.allowNetworkSet
}

func MergeConfigs(base, override *SandboxConfig) SandboxConfig {
	var merged SandboxConfig
	if base != nil {
		merged = base.Clone()
	}
	if override == nil {
		return merged
	}
	if override.Root != "" {
		merged.Root = override.Root
	}
	if override.VirtualRoot != "" {
		merged.VirtualRoot = override.VirtualRoot
	}
	if len(override.AllowPaths) > 0 {
		merged.AllowPaths = cloneStrings(override.AllowPaths)
	}
	if len(override.ReadonlyPaths) > 0 {
		merged.ReadonlyPaths = cloneStrings(override.ReadonlyPaths)
	}
	if len(override.BlockedCommands) > 0 {
		merged.BlockedCommands = cloneStrings(override.BlockedCommands)
	}
	if override.HasAllowNetworkOverride() {
		merged.SetAllowNetwork(override.AllowNetwork)
	}
	return merged
}

func (s *SandboxConfig) UnmarshalYAML(value *yaml.Node) error {
	type raw SandboxConfig
	var decoded raw
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	*s = SandboxConfig(decoded)
	s.allowNetworkSet = yamlMappingHasKey(value, "allow_network")
	return nil
}

func (s SandboxConfig) MarshalYAML() (any, error) {
	type raw struct {
		VirtualRoot     string   `yaml:"virtual_root,omitempty"`
		Root            string   `yaml:"root"`
		AllowPaths      []string `yaml:"allow_paths,omitempty"`
		ReadonlyPaths   []string `yaml:"readonly_paths,omitempty"`
		AllowNetwork    *bool    `yaml:"allow_network,omitempty"`
		BlockedCommands []string `yaml:"blocked_commands,omitempty"`
	}
	encoded := raw{
		VirtualRoot:     s.VirtualRoot,
		Root:            s.Root,
		AllowPaths:      cloneStrings(s.AllowPaths),
		ReadonlyPaths:   cloneStrings(s.ReadonlyPaths),
		BlockedCommands: cloneStrings(s.BlockedCommands),
	}
	if s.AllowNetwork || s.allowNetworkSet {
		allow := s.AllowNetwork
		encoded.AllowNetwork = &allow
	}
	return encoded, nil
}

func yamlMappingHasKey(node *yaml.Node, key string) bool {
	if node == nil || node.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return true
		}
	}
	return false
}

func (s *SandboxConfig) IsAllowed(path string, write bool) bool {
	if s == nil || (s.Root == "" && len(s.AllowPaths) == 0 && len(s.ReadonlyPaths) == 0) {
		return true
	}

	absPath, err := normalizePath(path)
	if err != nil {
		return false
	}

	readonlyMatched := false
	for _, rp := range s.ReadonlyPaths {
		absRP, err := normalizePath(rp)
		if err != nil {
			continue
		}
		if IsSubpath(absPath, absRP) {
			readonlyMatched = true
			if write {
				return false
			}
		}
	}
	if readonlyMatched && !write {
		return true
	}

	for _, ap := range s.AllowPaths {
		absAP, err := normalizePath(ap)
		if err != nil {
			continue
		}
		if IsSubpath(absPath, absAP) {
			return true
		}
	}

	if s.Root != "" {
		absRoot, err := normalizePath(s.Root)
		if err != nil {
			return false
		}
		if IsSubpath(absPath, absRoot) {
			if readonlyMatched && write {
				return false
			}
			return true
		}
	}

	return false
}

// AutoResolvePath resolves any path form (virtual, real, relative, absolute) into the sandbox.
// Path traversal (../) is blocked by ResolvePath's VirtualRoot prefix validation.
func (s *SandboxConfig) AutoResolvePath(path string) (string, error) {
	if s == nil || s.VirtualRoot == "" {
		return path, nil
	}

	nativePath := PathCleanVirtual(VirtualToNative(path))
	vr := PathCleanVirtual(s.VirtualRoot)
	sep := VirtualSeparator()

	if pathMatchesRoot(nativePath, vr, sep) {
		return s.ResolvePath(path)
	}

	if s.Root != "" {
		absRoot := filepath.Clean(s.Root)
		if pathMatchesRoot(nativePath, absRoot, string(filepath.Separator)) {
			rel := trimPrefixFold(nativePath, absRoot)
			if rel == "" || rel == string(filepath.Separator) {
				return s.ResolvePath(vr)
			}
			return s.ResolvePath(vr + VirtualToNative(rel))
		}
	}

	nativePathSlashes := filepath.ToSlash(nativePath)
	if filepath.IsAbs(nativePath) {
		if volume := filepath.VolumeName(nativePath); volume != "" {
			nativePathSlashes = strings.TrimPrefix(nativePathSlashes, filepath.ToSlash(volume))
			if nativePathSlashes == "" {
				nativePathSlashes = "/"
			}
		}
		if !strings.HasPrefix(nativePathSlashes, "/") {
			nativePathSlashes = "/" + strings.TrimLeft(nativePathSlashes, "/")
		}
		virtualPath := vr + VirtualToNative(nativePathSlashes)
		return s.ResolvePath(virtualPath)
	}

	virtualPath := vr + sep + VirtualToNative(nativePathSlashes)
	return s.ResolvePath(virtualPath)
}

// ResolvePath validates that path starts with VirtualRoot and translates it to the real filesystem path.
func (s *SandboxConfig) ResolvePath(path string) (string, error) {
	if s == nil || s.VirtualRoot == "" {
		return path, nil
	}

	cleaned := PathCleanVirtual(VirtualToNative(path))
	vr := PathCleanVirtual(s.VirtualRoot)
	sep := VirtualSeparator()

	if !pathMatchesRoot(cleaned, vr, sep) {
		return "", fmt.Errorf("path %q must start with %q (sandbox enforced)", path, s.VirtualRoot)
	}

	rel := trimPrefixFold(filepath.ToSlash(cleaned), filepath.ToSlash(vr))
	if rel == "" || rel == "/" {
		return s.Root, nil
	}
	return filepath.Join(s.Root, rel[1:]), nil
}

func (s *SandboxConfig) RewritePaths(text string) string {
	if s == nil || s.VirtualRoot == "" {
		return text
	}
	return strings.ReplaceAll(text, s.VirtualRoot, s.Root)
}

func (s *SandboxConfig) RewriteOutputPaths(text string) string {
	if s == nil || s.VirtualRoot == "" || s.Root == "" {
		return text
	}
	resolvedRoot := EvalSymlinks(s.Root)
	result := strings.ReplaceAll(text, resolvedRoot, s.VirtualRoot)
	if resolvedRoot != s.Root {
		result = strings.ReplaceAll(result, s.Root, s.VirtualRoot)
	}
	return result
}

func (s *SandboxConfig) RewriteError(err error) error {
	if s == nil || s.VirtualRoot == "" || err == nil {
		return err
	}
	return &rewrittenError{message: s.RewriteOutputPaths(err.Error()), cause: err}
}

func (s *SandboxConfig) RealToVirtual(realPath string) string {
	if s == nil || s.VirtualRoot == "" || s.Root == "" {
		return realPath
	}
	absPath, err := filepath.Abs(realPath)
	if err != nil {
		return PathJoinVirtual(s.VirtualRoot, "[external]", filepath.Base(realPath))
	}
	absPath = filepath.Clean(absPath)
	absRoot, err := filepath.Abs(s.Root)
	if err != nil {
		return PathJoinVirtual(s.VirtualRoot, "[external]", filepath.Base(realPath))
	}
	absRootSep := absRoot + string(filepath.Separator)
	if strings.EqualFold(absPath, absRoot) {
		return s.VirtualRoot
	}
	if pathMatchesRoot(absPath, absRoot, string(filepath.Separator)) {
		rel := filepath.ToSlash(trimPrefixFold(absPath, absRootSep))
		return PathJoinVirtual(s.VirtualRoot, VirtualToNative(rel))
	}
	return PathJoinVirtual(s.VirtualRoot, "[external]", filepath.Base(absPath))
}

func (s *SandboxConfig) ValidatePath(resolvedPath string) error {
	if s == nil || (s.Root == "" && len(s.AllowPaths) == 0 && len(s.ReadonlyPaths) == 0) {
		return nil
	}
	if s.IsAllowed(resolvedPath, false) {
		return nil
	}
	return fmt.Errorf("path %q is outside sandbox policy", resolvedPath)
}

func (s *SandboxConfig) IsBlockedCommand(cmd string) bool {
	for _, segment := range ShellCommandSegments(cmd) {
		trimmed := strings.TrimSpace(segment)
		if trimmed == "" {
			continue
		}

		fields := strings.Fields(trimmed)
		baseCmd := ""
		if len(fields) > 0 {
			baseCmd = filepath.Base(fields[0])
		}

		for _, blocked := range s.BlockedCommands {
			if baseCmd == blocked || trimmed == blocked || strings.HasPrefix(trimmed, blocked+" ") || strings.HasPrefix(trimmed, blocked+"\t") {
				return true
			}
			if (strings.Contains(blocked, " ") || strings.Contains(blocked, "=")) && strings.HasPrefix(trimmed, blocked) {
				return true
			}
		}
	}
	return false
}
