package completion

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	httpclient "raind/internal/raind/core/client"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
)

type directive string

const (
	directiveNoFile  directive = ":nofile"
	directiveDefault directive = ":default"
	directiveDirs    directive = ":dirs"
)

type completionResult struct {
	items     []string
	directive directive
}

func CommandComplete(app *cli.App) *cli.Command {
	return &cli.Command{
		Name:            "__complete",
		Hidden:          true,
		SkipFlagParsing: true,
		Action: func(ctx *cli.Context) error {
			result := complete(app, ctx.Args().Slice())
			for _, item := range result.items {
				fmt.Fprintln(ctx.App.Writer, item)
			}
			fmt.Fprintln(ctx.App.Writer, result.directive)
			return nil
		},
	}
}

func complete(app *cli.App, args []string) completionResult {
	current, completed := splitCompletionArgs(args)
	if strings.HasPrefix(current, "-") {
		return completionResult{items: completeFlags(commandAt(app.Commands, completed), current), directive: directiveNoFile}
	}

	state := resolveCommand(app.Commands, completed)
	if state.command == nil {
		return completionResult{items: filterPrefix(commandNames(app.Commands), current), directive: directiveNoFile}
	}
	if len(state.remaining) == 0 && len(state.command.Subcommands) > 0 {
		return completionResult{items: filterPrefix(commandNames(state.command.Subcommands), current), directive: directiveNoFile}
	}

	return completeArguments(state, current)
}

func splitCompletionArgs(args []string) (string, []string) {
	if len(args) == 0 {
		return "", nil
	}
	return args[len(args)-1], args[:len(args)-1]
}

type commandState struct {
	path      []string
	command   *cli.Command
	remaining []string
}

func resolveCommand(commands []*cli.Command, words []string) commandState {
	state := commandState{}
	currentCommands := commands
	for len(words) > 0 {
		word := words[0]
		if strings.HasPrefix(word, "-") {
			words = skipFlagValue(state.command, words)
			continue
		}
		cmd := findCommand(currentCommands, word)
		if cmd == nil {
			break
		}
		state.path = append(state.path, cmd.Name)
		state.command = cmd
		currentCommands = cmd.Subcommands
		words = words[1:]
	}
	state.remaining = words
	return state
}

func commandAt(commands []*cli.Command, words []string) *cli.Command {
	return resolveCommand(commands, words).command
}

func skipFlagValue(cmd *cli.Command, words []string) []string {
	if len(words) == 0 {
		return words
	}
	word := words[0]
	if strings.Contains(word, "=") || !flagNeedsValue(cmd, word) || len(words) == 1 {
		return words[1:]
	}
	return words[2:]
}

func flagNeedsValue(cmd *cli.Command, word string) bool {
	if cmd == nil {
		return false
	}
	name := strings.TrimLeft(strings.SplitN(word, "=", 2)[0], "-")
	for _, flag := range cmd.Flags {
		if flag == nil {
			continue
		}
		for _, flagName := range flag.Names() {
			if flagName == name {
				return flagTakesValue(flag)
			}
		}
	}
	return false
}

func findCommand(commands []*cli.Command, name string) *cli.Command {
	for _, cmd := range commands {
		if !isCompletableCommand(cmd) {
			continue
		}
		if cmd.Name == name {
			return cmd
		}
		for _, alias := range cmd.Aliases {
			if alias == name {
				return cmd
			}
		}
	}
	return nil
}

func commandNames(commands []*cli.Command) []string {
	var names []string
	for _, cmd := range commands {
		if !isCompletableCommand(cmd) {
			continue
		}
		names = append(names, cmd.Name)
		names = append(names, cmd.Aliases...)
	}
	return uniqueStrings(names)
}

func isCompletableCommand(cmd *cli.Command) bool {
	return cmd != nil && !cmd.Hidden && cmd.Name != "help"
}

func completeFlags(cmd *cli.Command, current string) []string {
	var forms []string
	if cmd != nil {
		forms = append(forms, collectFlagForms(collectFlags(cmd.Flags))...)
	}
	forms = append(forms, "--help", "-h")
	return filterPrefix(uniqueStrings(forms), current)
}

func completeArguments(state commandState, current string) completionResult {
	switch strings.Join(state.path, " ") {
	case "resource apply":
		return completionResult{directive: directiveDefault}
	case "resource delete":
		if flagValuePending(state.command, state.remaining, "file") {
			return completionResult{directive: directiveDefault}
		}
		return completeResourceKindName(state.remaining, current, true)
	case "resource get":
		return completeResourceKindName(state.remaining, current, false)
	case "resource describe", "resource scale":
		return completeResourceKindName(state.remaining, current, true)
	case "resource create":
		if len(state.remaining) == 0 {
			return completionResult{items: filterPrefix([]string{"namespace", "ns"}, current), directive: directiveNoFile}
		}
		return completionResult{directive: directiveNoFile}
	case "container start", "container stop", "container rm", "container attach", "container exec", "container logs", "container inspect":
		return completionResult{items: filterPrefix(completeAPIPath("/v1/containers", "", "containerId", "name"), current), directive: directiveNoFile}
	case "image rm":
		return completionResult{items: filterPrefix(completeImages(), current), directive: directiveNoFile}
	case "bottle start", "bottle stop", "bottle rm", "bottle show":
		return completionResult{items: filterPrefix(completeAPIPath("/v1/bottle", "bottles", "bottleName", "bottleId"), current), directive: directiveNoFile}
	}

	if len(state.command.Subcommands) > 0 && len(state.remaining) == 0 {
		return completionResult{items: filterPrefix(commandNames(state.command.Subcommands), current), directive: directiveNoFile}
	}
	return completionResult{directive: directiveDefault}
}

func flagValuePending(cmd *cli.Command, words []string, name string) bool {
	if len(words) == 0 {
		return false
	}
	last := words[len(words)-1]
	if !strings.HasPrefix(last, "-") || strings.Contains(last, "=") {
		return false
	}
	lastName := strings.TrimLeft(last, "-")
	if lastName == name {
		return true
	}
	if cmd == nil {
		return false
	}
	for _, flag := range cmd.Flags {
		if flag == nil {
			continue
		}
		names := flag.Names()
		if !contains(names, name) || !contains(names, lastName) {
			continue
		}
		return flagTakesValue(flag)
	}
	return false
}

func completeResourceKindName(args []string, current string, includeNames bool) completionResult {
	if len(args) == 0 {
		return completionResult{items: filterPrefix(resourceKinds(), current), directive: directiveNoFile}
	}
	if !includeNames || len(args) > 1 {
		return completionResult{directive: directiveNoFile}
	}
	kind := normalizeResourceKind(args[0])
	return completionResult{items: filterPrefix(completeResourceNames(kind), current), directive: directiveNoFile}
}

func resourceKinds() []string {
	return []string{
		"configmap",
		"cm",
		"deployment",
		"deploy",
		"ing",
		"ingress",
		"namespace",
		"ns",
		"networkpolicy",
		"netpol",
		"pod",
		"po",
		"pvc",
		"replicaset",
		"rs",
		"secret",
		"service",
		"svc",
	}
}

func normalizeResourceKind(kind string) string {
	switch strings.ToLower(kind) {
	case "po", "pod", "pods":
		return "pod"
	case "rs", "replicaset", "replicasets":
		return "replicaset"
	case "deploy", "deployment", "deployments":
		return "deployment"
	case "svc", "service", "services":
		return "service"
	case "ing", "ingress", "ingresses":
		return "ingress"
	case "ns", "namespace", "namespaces":
		return "namespace"
	case "cm", "configmap", "configmaps":
		return "configmap"
	case "secret", "secrets":
		return "secret"
	case "netpol", "networkpolicy", "networkpolicies":
		return "networkpolicy"
	case "pvc", "persistentvolumeclaim", "persistentvolumeclaims":
		return "pvc"
	default:
		return kind
	}
}

func completeResourceNames(kind string) []string {
	switch kind {
	case "pod":
		return completeAPIPath("/v1/pods", "", "name", "podId")
	case "replicaset":
		return completeAPIPath("/v1/replicasets", "", "name", "replicaSetId")
	case "deployment":
		return completeAPIPath("/v1/deployments", "", "name", "deploymentId")
	case "service":
		return completeAPIPath("/v1/services", "", "name", "serviceId")
	case "ingress":
		return completeAPIPath("/v1/ingresses", "", "name", "ingressId")
	case "namespace":
		return completeAPIPath("/v1/namespaces", "", "name")
	case "configmap":
		return completeAPIPath("/v1/configmaps", "", "name", "configMapId")
	case "secret":
		return completeAPIPath("/v1/secrets", "", "name", "secretId")
	case "networkpolicy":
		return completeAPIPath("/v1/networkpolicies", "", "name", "networkPolicyId")
	case "pvc":
		return completeAPIPath("/v1/persistentvolumeclaims", "", "name", "pvcId")
	default:
		return nil
	}
}

func completeImages() []string {
	items := completeAPIPath("/v1/images", "", "repository")
	var resp struct {
		Data []struct {
			Repository string `json:"repository"`
			Reference  string `json:"reference"`
		} `json:"data"`
	}
	if err := getJSON("/v1/images", &resp); err != nil {
		return items
	}
	for _, image := range resp.Data {
		if image.Repository != "" && image.Reference != "" {
			items = append(items, image.Repository+":"+image.Reference)
		}
	}
	return uniqueStrings(items)
}

func completeAPIPath(path string, nested string, fields ...string) []string {
	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	if err := getJSON(path, &resp); err != nil {
		return nil
	}

	raw := resp.Data
	if nested != "" {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(resp.Data, &obj); err != nil {
			return nil
		}
		raw = obj[nested]
	}

	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil
	}

	var items []string
	for _, row := range rows {
		for _, field := range fields {
			value, ok := row[field].(string)
			if ok && strings.TrimSpace(value) != "" {
				items = append(items, value)
			}
		}
	}
	return uniqueStrings(items)
}

func getJSON(path string, out any) error {
	client, err := httpclient.NewHttpClient()
	if err != nil {
		return err
	}
	client.Client.Timeout = 700 * time.Millisecond
	if err := client.NewRequest(http.MethodGet, pathWithEscapedQuery(path), nil); err != nil {
		return err
	}
	resp, err := client.Client.Do(client.Request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if !client.IsStatusOk(resp) {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func pathWithEscapedQuery(path string) string {
	if !strings.Contains(path, "?") {
		return path
	}
	parts := strings.SplitN(path, "?", 2)
	values, err := url.ParseQuery(parts[1])
	if err != nil {
		return path
	}
	return parts[0] + "?" + values.Encode()
}

func filterPrefix(items []string, prefix string) []string {
	var out []string
	for _, item := range items {
		if strings.HasPrefix(item, prefix) {
			out = append(out, item)
		}
	}
	return uniqueStrings(out)
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
