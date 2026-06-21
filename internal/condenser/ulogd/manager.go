package ulogd

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"raind/internal/condenser/utils"
)

const (
	DefaultConfigPath = "/etc/ulogd.conf"

	beginMarker = "# BEGIN RAIND MANAGED NFLOG CONFIG"
	endMarker   = "# END RAIND MANAGED NFLOG CONFIG"

	pluginBeginMarker = "# BEGIN RAIND MANAGED ULOGD PLUGINS"
	pluginEndMarker   = "# END RAIND MANAGED ULOGD PLUGINS"
)

var requiredPlugins = []string{
	"ulogd_inppkt_NFLOG.so",
	"ulogd_filter_IFINDEX.so",
	"ulogd_filter_IP2STR.so",
	"ulogd_filter_PRINTPKT.so",
	"ulogd_raw2packet_BASE.so",
	"ulogd_output_JSON.so",
}

// Manager maintains the ulogd configuration needed for Raind netflow logs.
//
// It only manages the Raind marker blocks. Existing ulogd configuration outside
// those blocks is preserved as-is.
type Manager struct {
	filesystemHandler utils.FilesystemHandler
	commandFactory    utils.CommandFactory

	configPath string
	backupPath string
	outputPath string
}

func NewManager() *Manager {
	return &Manager{
		filesystemHandler: utils.NewFilesystemExecutor(),
		commandFactory:    utils.NewCommandFactory(),
		configPath:        DefaultConfigPath,
		backupPath:        DefaultConfigPath + ".raind.bak",
		outputPath:        utils.UlogPath,
	}
}

// EnsureRaindNFLOGConfig best-effort configures ulogd to collect the NFLOG
// groups used by Raind security policy logging.
//
// It is intentionally conservative:
//   - it can be disabled with RAIND_AUTO_CONFIG_ULOGD=false
//   - it does not create a new /etc/ulogd.conf from scratch
//   - it only updates the Raind managed blocks
//   - plugin lines are only added when the plugin file exists locally and is
//     not already enabled outside the Raind managed block
//   - restart failures are returned to the caller but should not be fatal to
//     condenser startup
func (m *Manager) EnsureRaindNFLOGConfig() error {
	if autoConfigDisabled() {
		log.Printf("ulogd auto-config skipped: RAIND_AUTO_CONFIG_ULOGD is disabled")
		return nil
	}

	if _, err := exec.LookPath("ulogd"); err != nil {
		return fmt.Errorf("ulogd executable not found: %w", err)
	}

	data, err := m.filesystemHandler.ReadFile(m.configPath)
	if err != nil {
		if m.filesystemHandler.IsNotExist(err) {
			return fmt.Errorf("ulogd config not found at %s", m.configPath)
		}
		return fmt.Errorf("read ulogd config: %w", err)
	}

	pluginPaths, err := m.detectPluginPaths()
	if err != nil {
		return err
	}

	if err := m.filesystemHandler.MkdirAll(filepath.Dir(m.outputPath), 0o755); err != nil {
		return fmt.Errorf("create ulogd output directory: %w", err)
	}

	current := string(data)
	withoutBlocks := removeManagedBlocks(current)
	for _, group := range []string{"10", "11", "12"} {
		if configMentionsNFLOGGroup(withoutBlocks, group) {
			log.Printf("ulogd auto-config warning: NFLOG group %s appears outside the Raind managed block", group)
		}
	}

	pluginBlock := renderPluginBlock(pluginPaths, withoutBlocks)
	nflogBlock := renderManagedBlock(m.outputPath)

	updated, changed := upsertRaindConfig(withoutBlocks, pluginBlock, nflogBlock)
	if !changed {
		log.Printf("ulogd auto-config: Raind NFLOG configuration already up to date")
		return nil
	}

	if err := m.backupOriginalConfig(data); err != nil {
		return fmt.Errorf("backup original ulogd config: %w", err)
	}

	if err := m.filesystemHandler.WriteFile(m.configPath, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write ulogd config: %w", err)
	}
	log.Printf("ulogd auto-config: updated Raind NFLOG configuration in %s", m.configPath)

	if err := m.reloadOrRestartUlogd(); err != nil {
		return fmt.Errorf("reload/restart ulogd after config update: %w", err)
	}
	return nil
}

func (m *Manager) backupOriginalConfig(data []byte) error {
	if strings.TrimSpace(m.backupPath) == "" {
		return nil
	}

	if _, err := m.filesystemHandler.ReadFile(m.backupPath); err == nil {
		log.Printf("ulogd auto-config: original config backup already exists at %s", m.backupPath)
		return nil
	} else if !m.filesystemHandler.IsNotExist(err) {
		return fmt.Errorf("check existing backup %s: %w", m.backupPath, err)
	}

	if err := m.filesystemHandler.WriteFile(m.backupPath, data, 0o644); err != nil {
		return fmt.Errorf("write backup %s: %w", m.backupPath, err)
	}
	log.Printf("ulogd auto-config: backed up original config to %s", m.backupPath)
	return nil
}

func autoConfigDisabled() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("RAIND_AUTO_CONFIG_ULOGD")))
	switch v {
	case "", "1", "true", "yes", "on":
		return false
	case "0", "false", "no", "off", "disabled", "disable":
		return true
	default:
		return false
	}
}

func (m *Manager) detectPluginPaths() (map[string]string, error) {
	candidates := make([]string, 0, 8)

	if out, err := m.commandFactory.Command("dpkg-architecture", "-qDEB_HOST_MULTIARCH").Output(); err == nil {
		multiarch := strings.TrimSpace(string(out))
		if multiarch != "" {
			candidates = append(candidates, filepath.Join("/usr/lib", multiarch, "ulogd"))
		}
	}

	if matches, err := filepath.Glob("/usr/lib/*/ulogd"); err == nil {
		candidates = append(candidates, matches...)
	}
	candidates = append(candidates, "/usr/lib/ulogd", "/usr/lib64/ulogd")

	seen := map[string]struct{}{}
	var warnings []string
	for _, dir := range candidates {
		if dir == "" {
			continue
		}
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}
		paths, missing := pluginPathsInDir(dir)
		if len(missing) == 0 {
			return paths, nil
		}
		if len(paths) > 0 {
			warnings = append(warnings, fmt.Sprintf("%s missing %s", dir, strings.Join(missing, ", ")))
		}
	}

	if len(warnings) > 0 {
		for _, warning := range warnings {
			log.Printf("ulogd auto-config warning: required plugin not found: %s", warning)
		}
	}
	return nil, errors.New("required ulogd plugins not found")
}

func pluginPathsInDir(dir string) (map[string]string, []string) {
	paths := make(map[string]string, len(requiredPlugins))
	var missing []string
	for _, plugin := range requiredPlugins {
		path := filepath.Join(dir, plugin)
		if _, err := os.Stat(path); err != nil {
			missing = append(missing, plugin)
			continue
		}
		paths[plugin] = path
	}
	return paths, missing
}

func renderPluginBlock(pluginPaths map[string]string, existingConfigWithoutBlocks string) string {
	var lines []string
	for _, plugin := range requiredPlugins {
		pluginPath := pluginPaths[plugin]
		if pluginPath == "" {
			continue
		}
		if configMentionsPlugin(existingConfigWithoutBlocks, plugin) {
			continue
		}
		lines = append(lines, fmt.Sprintf("plugin=\"%s\"", pluginPath))
	}
	if len(lines) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(pluginBeginMarker + "\n")
	b.WriteString("# This block is managed by Raind. Manual changes inside this block may be overwritten.\n")
	b.WriteString(strings.Join(lines, "\n"))
	b.WriteString("\n" + pluginEndMarker)
	return b.String()
}

func renderManagedBlock(outputPath string) string {
	var b strings.Builder
	b.WriteString(beginMarker + "\n")
	b.WriteString("# This block is managed by Raind. Manual changes inside this block may be overwritten.\n")
	b.WriteString("# It configures ulogd to collect Raind NFLOG groups 10, 11, and 12.\n\n")

	for _, group := range []string{"10", "11", "12"} {
		b.WriteString(fmt.Sprintf("stack=raind_log%s:NFLOG,raind_base:BASE,raind_ifi:IFINDEX,raind_ip2str:IP2STR,raind_print:PRINTPKT,raind_json:JSON\n", group))
	}

	b.WriteString("\n# ---- Raind inputs (NFLOG groups) ----\n")
	for _, group := range []string{"10", "11", "12"} {
		b.WriteString(fmt.Sprintf("[raind_log%s]\ngroup=%s\n\n", group, group))
	}

	b.WriteString("# ---- Raind common instances ----\n")
	b.WriteString("[raind_base]\n")
	b.WriteString("[raind_ifi]\n")
	b.WriteString("[raind_ip2str]\n")
	b.WriteString("[raind_print]\n\n")
	b.WriteString("[raind_json]\n")
	b.WriteString(fmt.Sprintf("file=\"%s\"\n", outputPath))
	b.WriteString("sync=1\n")

	b.WriteString("\n" + endMarker)
	return b.String()
}

func configMentionsPlugin(config, plugin string) bool {
	for _, line := range strings.Split(config, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.Contains(trimmed, "plugin=") && strings.Contains(trimmed, plugin) {
			return true
		}
	}
	return false
}

func configMentionsNFLOGGroup(config, group string) bool {
	for _, line := range strings.Split(config, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.ReplaceAll(trimmed, " ", "") == "group="+group {
			return true
		}
	}
	return false
}

func removeManagedBlocks(config string) string {
	withoutPlugins := removeMarkedBlock(config, pluginBeginMarker, pluginEndMarker)
	return removeMarkedBlock(withoutPlugins, beginMarker, endMarker)
}

func removeManagedBlock(config string) string {
	return removeMarkedBlock(config, beginMarker, endMarker)
}

func removeMarkedBlock(config, startMarker, endMarker string) string {
	start := strings.Index(config, startMarker)
	if start == -1 {
		return config
	}
	end := strings.Index(config[start:], endMarker)
	if end == -1 {
		return config
	}
	endAbs := start + end + len(endMarker)
	trimLeft := config[:start]
	trimRight := config[endAbs:]
	return strings.TrimRight(trimLeft, "\n") + "\n" + strings.TrimLeft(trimRight, "\n")
}

func upsertRaindConfig(config, pluginBlock, nflogBlock string) (string, bool) {
	updated := config
	if pluginBlock != "" {
		updated = insertPluginBlock(updated, pluginBlock)
	}
	updated = insertNFLOGBlock(updated, nflogBlock)
	if !strings.HasSuffix(updated, "\n") {
		updated += "\n"
	}
	return updated, updated != config
}

func upsertManagedBlock(config, block string) (string, bool) {
	withoutBlock := removeManagedBlock(config)
	updated := insertNFLOGBlock(withoutBlock, block)
	if !strings.HasSuffix(updated, "\n") {
		updated += "\n"
	}
	return updated, updated != config
}

func insertPluginBlock(config, block string) string {
	idx := insertionPointAfterLastPlugin(config)
	return insertBlockAt(config, block, idx)
}

func insertNFLOGBlock(config, block string) string {
	idx := insertionPointBeforeFirstSectionAfterStack(config)
	if idx == -1 {
		idx = insertionPointAfterLastStack(config)
	}
	if idx == -1 {
		idx = insertionPointAfterLastPlugin(config)
	}
	return insertBlockAt(config, block, idx)
}

func insertionPointAfterLastPlugin(config string) int {
	last := -1
	offset := 0
	for _, line := range splitLinesAfter(config) {
		trimmed := strings.TrimSpace(line.text)
		if strings.HasPrefix(trimmed, "plugin=") || strings.HasPrefix(trimmed, "#plugin=") {
			last = offset + len(line.text)
		}
		offset += len(line.text)
	}
	return last
}

func insertionPointAfterLastStack(config string) int {
	last := -1
	foundStackCluster := false
	offset := 0
	sectionRe := regexp.MustCompile(`^\s*\[[^\]]+\]\s*$`)
	for _, line := range splitLinesAfter(config) {
		trimmed := strings.TrimSpace(line.text)
		if strings.HasPrefix(trimmed, "stack=") || strings.HasPrefix(trimmed, "#stack=") {
			foundStackCluster = true
			last = offset + len(line.text)
			offset += len(line.text)
			continue
		}
		if foundStackCluster && trimmed != "" && !strings.HasPrefix(trimmed, "#") && sectionRe.MatchString(line.text) {
			return last
		}
		offset += len(line.text)
	}
	return last
}

func insertionPointBeforeFirstSectionAfterStack(config string) int {
	foundStackCluster := false
	offset := 0
	sectionRe := regexp.MustCompile(`^\s*\[[^\]]+\]\s*$`)
	for _, line := range splitLinesAfter(config) {
		trimmed := strings.TrimSpace(line.text)
		if strings.HasPrefix(trimmed, "stack=") || strings.HasPrefix(trimmed, "#stack=") {
			foundStackCluster = true
			offset += len(line.text)
			continue
		}
		if !foundStackCluster {
			offset += len(line.text)
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			offset += len(line.text)
			continue
		}
		if sectionRe.MatchString(line.text) {
			return offset
		}
		offset += len(line.text)
	}
	return -1
}

type configLine struct {
	text string
}

func splitLinesAfter(config string) []configLine {
	parts := strings.SplitAfter(config, "\n")
	lines := make([]configLine, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		lines = append(lines, configLine{text: part})
	}
	return lines
}

func insertBlockAt(config, block string, idx int) string {
	if idx < 0 || idx > len(config) {
		idx = len(config)
	}

	before := config[:idx]
	after := config[idx:]
	var b strings.Builder
	b.WriteString(strings.TrimRight(before, "\n"))
	b.WriteString("\n\n")
	b.WriteString(block)
	b.WriteString("\n\n")
	b.WriteString(strings.TrimLeft(after, "\n"))
	return b.String()
}

func (m *Manager) reloadOrRestartUlogd() error {
	var errs []string
	commands := [][]string{
		{"systemctl", "reload", "ulogd"},
		{"systemctl", "restart", "ulogd"},
		{"service", "ulogd", "restart"},
	}
	for _, command := range commands {
		name, args := command[0], command[1:]
		if err := m.commandFactory.Command(name, args...).Run(); err == nil {
			log.Printf("ulogd auto-config: %s %s succeeded", name, strings.Join(args, " "))
			return nil
		} else {
			errs = append(errs, fmt.Sprintf("%s %s: %v", name, strings.Join(args, " "), err))
		}
	}
	return errors.New(strings.Join(errs, "; "))
}
