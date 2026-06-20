package ulogd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raind/internal/condenser/utils"
)

func TestUpsertManagedBlockAddsBlockBeforeFirstInstanceSectionAfterStacks(t *testing.T) {
	config := `[global]
plugin="/usr/lib/example/ulogd/ulogd_inppkt_NFLOG.so"

stack=log10:NFLOG,base:BASE,json:JSON
stack=log11:NFLOG,base:BASE,json:JSON

[log10]
group=10
`
	block := beginMarker + "\nmanaged\n" + endMarker

	updated, changed := upsertManagedBlock(config, block)
	if !changed {
		t.Fatal("expected config to change")
	}
	if !strings.Contains(updated, block) {
		t.Fatalf("expected managed block in config: %s", updated)
	}
	if strings.Index(updated, block) > strings.Index(updated, "[log10]") {
		t.Fatalf("expected managed block before first instance section after stacks: %s", updated)
	}
	if !strings.Contains(updated, "[global]") {
		t.Fatalf("expected existing config to be preserved: %s", updated)
	}
}

func TestUpsertManagedBlockReplacesOnlyManagedBlock(t *testing.T) {
	oldBlock := beginMarker + "\nold\n" + endMarker
	newBlock := beginMarker + "\nnew\n" + endMarker
	config := "before\n" + oldBlock + "\nafter\n"

	updated, changed := upsertManagedBlock(config, newBlock)
	if !changed {
		t.Fatal("expected config to change")
	}
	if strings.Contains(updated, "old") {
		t.Fatalf("expected old managed block to be removed: %s", updated)
	}
	if !strings.Contains(updated, "before") || !strings.Contains(updated, "after") {
		t.Fatalf("expected non-managed config to be preserved: %s", updated)
	}
	if !strings.Contains(updated, "new") {
		t.Fatalf("expected new managed block: %s", updated)
	}
}

func TestRenderPluginBlockSkipsPluginsAlreadyConfiguredOutsideBlock(t *testing.T) {
	pluginPaths := map[string]string{}
	for _, plugin := range requiredPlugins {
		pluginPaths[plugin] = filepath.Join("/usr/lib/example/ulogd", plugin)
	}
	config := "plugin=\"/usr/lib/example/ulogd/ulogd_inppkt_NFLOG.so\"\n"
	block := renderPluginBlock(pluginPaths, config)

	if strings.Contains(block, "plugin=\"/usr/lib/example/ulogd/ulogd_inppkt_NFLOG.so\"") {
		t.Fatalf("expected existing active plugin to be skipped: %s", block)
	}
	if !strings.Contains(block, "ulogd_output_JSON.so") {
		t.Fatalf("expected missing active plugins to be added: %s", block)
	}
}

func TestRenderPluginBlockDoesNotSkipCommentedPlugins(t *testing.T) {
	pluginPaths := map[string]string{}
	for _, plugin := range requiredPlugins {
		pluginPaths[plugin] = filepath.Join("/usr/lib/example/ulogd", plugin)
	}
	config := "#plugin=\"/usr/lib/example/ulogd/ulogd_output_JSON.so\"\n"
	block := renderPluginBlock(pluginPaths, config)

	if !strings.Contains(block, "plugin=\"/usr/lib/example/ulogd/ulogd_output_JSON.so\"") {
		t.Fatalf("expected commented plugin to be enabled by Raind block: %s", block)
	}
}

func TestRenderManagedBlockIncludesStacksBeforeInstances(t *testing.T) {
	block := renderManagedBlock("/var/log/ulog/raind.jsonl")

	if !strings.Contains(block, "raind_log10:NFLOG") || !strings.Contains(block, "group=10") {
		t.Fatalf("expected group 10 stack and instance: %s", block)
	}
	if strings.Index(block, "stack=raind_log10") > strings.Index(block, "[raind_log10]") {
		t.Fatalf("expected stack definitions before instance sections: %s", block)
	}
	if !strings.Contains(block, "file=\"/var/log/ulog/raind.jsonl\"") {
		t.Fatalf("expected output path: %s", block)
	}
}

func TestUpsertRaindConfigInsertsPluginBlockAfterPluginLinesAndNFLOGBlockBeforeInstances(t *testing.T) {
	config := `[global]
# PLUGIN OPTIONS
plugin="/usr/lib/example/ulogd/ulogd_inppkt_NFLOG.so"
#plugin="/usr/lib/example/ulogd/ulogd_output_JSON.so"

stack=log10:NFLOG,base:BASE,ifi:IFINDEX,ip2str:IP2STR,print:PRINTPKT,json:JSON

[log10]
group=10
`
	pluginBlock := pluginBeginMarker + "\nplugin=\"/usr/lib/example/ulogd/ulogd_output_JSON.so\"\n" + pluginEndMarker
	nflogBlock := beginMarker + "\nstack=raind_log10:NFLOG,raind_base:BASE,raind_json:JSON\n\n[raind_log10]\ngroup=10\n" + endMarker

	updated, changed := upsertRaindConfig(config, pluginBlock, nflogBlock)
	if !changed {
		t.Fatal("expected config to change")
	}
	if strings.Index(updated, pluginBeginMarker) < strings.Index(updated, "plugin=\"/usr/lib/example/ulogd/ulogd_inppkt_NFLOG.so\"") {
		t.Fatalf("expected plugin block after plugin declarations: %s", updated)
	}
	if strings.Index(updated, beginMarker) > strings.Index(updated, "[log10]") {
		t.Fatalf("expected NFLOG block before existing instance sections: %s", updated)
	}
}

func TestConfigMentionsNFLOGGroupIgnoresComments(t *testing.T) {
	config := "# group=10\n[log11]\ngroup = 11\n"
	if configMentionsNFLOGGroup(config, "10") {
		t.Fatal("expected commented group to be ignored")
	}
	if !configMentionsNFLOGGroup(config, "11") {
		t.Fatal("expected group 11 to be detected")
	}
}

func TestBackupOriginalConfigCreatesBackupWithoutOverwriting(t *testing.T) {
	dir := t.TempDir()
	backupPath := filepath.Join(dir, "ulogd.conf.raind.bak")
	m := &Manager{
		filesystemHandler: utils.NewFilesystemExecutor(),
		backupPath:        backupPath,
	}

	if err := m.backupOriginalConfig([]byte("original\n")); err != nil {
		t.Fatalf("expected backup to be created: %v", err)
	}
	data, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("expected backup file: %v", err)
	}
	if string(data) != "original\n" {
		t.Fatalf("expected original data in backup, got %q", string(data))
	}

	if err := m.backupOriginalConfig([]byte("modified\n")); err != nil {
		t.Fatalf("expected existing backup to be preserved: %v", err)
	}
	data, err = os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("expected backup file: %v", err)
	}
	if string(data) != "original\n" {
		t.Fatalf("expected existing backup to be preserved, got %q", string(data))
	}
}
