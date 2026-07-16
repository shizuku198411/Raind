package completion

import (
	"fmt"
	"sort"
	"strings"

	"github.com/urfave/cli/v2"
)

type subcommandInfo struct {
	Name        string
	Aliases     []string
	Flags       []flagInfo
	Subcommands []subcommandInfo
}

type topCommandInfo struct {
	Name        string
	Aliases     []string
	Flags       []flagInfo
	Subcommands []subcommandInfo
}

type flagInfo struct {
	Names        []string
	TakesValue   bool
	DisplayForms []string
}

func CommandCompletion(app *cli.App) *cli.Command {
	return &cli.Command{
		Name:      "completion",
		Usage:     "print shell completion script",
		ArgsUsage: "bash|zsh|fish",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "snap", Usage: "render bash completion for snapd completer integration"},
		},
		Action: func(ctx *cli.Context) error {
			shell := strings.ToLower(strings.TrimSpace(ctx.Args().First()))
			if shell == "" {
				return fmt.Errorf("shell is required: bash|zsh|fish")
			}

			info := collectInfo(app)
			switch shell {
			case "bash":
				if ctx.Bool("snap") {
					fmt.Fprint(ctx.App.Writer, renderBashWithCommand(app.Name, info, "snap run "+app.Name, []string{app.Name, "/snap/bin/" + app.Name}))
				} else {
					fmt.Fprint(ctx.App.Writer, renderBash(app.Name, info))
				}
			case "zsh":
				fmt.Fprint(ctx.App.Writer, renderZsh(app.Name, info))
			case "fish":
				fmt.Fprint(ctx.App.Writer, renderFish(app.Name, info))
			default:
				return fmt.Errorf("unsupported shell: %s", shell)
			}

			return nil
		},
	}
}

func collectInfo(app *cli.App) []topCommandInfo {
	var out []topCommandInfo
	for _, cmd := range app.Commands {
		if cmd == nil || cmd.Hidden {
			continue
		}
		if cmd.Name == "completion" {
			continue
		}
		top := topCommandInfo{
			Name:    cmd.Name,
			Aliases: cmd.Aliases,
			Flags:   collectFlags(cmd.Flags),
		}
		for _, sub := range cmd.Subcommands {
			if sub == nil || sub.Hidden {
				continue
			}
			subInfo := collectSubcommandInfo(sub)
			top.Subcommands = append(top.Subcommands, subInfo)
		}
		sortSubcommands(top.Subcommands)
		out = append(out, top)
	}

	sortTopCommands(out)
	return out
}

func collectSubcommandInfo(cmd *cli.Command) subcommandInfo {
	info := subcommandInfo{
		Name:    cmd.Name,
		Aliases: cmd.Aliases,
		Flags:   collectFlags(cmd.Flags),
	}
	for _, sub := range cmd.Subcommands {
		if sub == nil || sub.Hidden {
			continue
		}
		info.Subcommands = append(info.Subcommands, collectSubcommandInfo(sub))
	}
	sortSubcommands(info.Subcommands)
	return info
}

func collectFlags(flags []cli.Flag) []flagInfo {
	var out []flagInfo
	for _, flag := range flags {
		if flag == nil {
			continue
		}
		names := uniqueStrings(flag.Names())
		if len(names) == 0 {
			continue
		}
		fi := flagInfo{
			Names:      names,
			TakesValue: flagTakesValue(flag),
		}
		fi.DisplayForms = flagDisplayForms(names)
		out = append(out, fi)
	}

	if len(out) == 0 {
		return out
	}

	sort.SliceStable(out, func(i, j int) bool {
		return strings.Join(out[i].DisplayForms, " ") < strings.Join(out[j].DisplayForms, " ")
	})
	return out
}

func flagTakesValue(flag cli.Flag) bool {
	if dg, ok := flag.(interface{ TakesValue() bool }); ok {
		return dg.TakesValue()
	}
	return false
}

func flagDisplayForms(names []string) []string {
	var forms []string
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if len(name) == 1 {
			forms = append(forms, "-"+name)
			continue
		}
		forms = append(forms, "--"+name)
	}
	return uniqueStrings(forms)
}

func sortTopCommands(cmds []topCommandInfo) {
	sort.SliceStable(cmds, func(i, j int) bool {
		return cmds[i].Name < cmds[j].Name
	})
}

func sortSubcommands(cmds []subcommandInfo) {
	sort.SliceStable(cmds, func(i, j int) bool {
		return cmds[i].Name < cmds[j].Name
	})
}

func uniqueStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	var out []string
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func renderBash(bin string, commands []topCommandInfo) string {
	return renderBashWithCommand(bin, commands, bin, []string{bin})
}

func renderBashWithCommand(bin string, commands []topCommandInfo, completeCommand string, completeNames []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# bash completion for %s\n", bin)
	fmt.Fprintf(&b, "_%s_complete() {\n", bin)
	fmt.Fprint(&b, "  local cur out directive candidates\n")
	fmt.Fprint(&b, "  COMPREPLY=()\n")
	fmt.Fprint(&b, "  cur=\"${COMP_WORDS[COMP_CWORD]}\"\n")
	fmt.Fprintf(&b, "  out=$(%s __complete \"${COMP_WORDS[@]:1}\" 2>/dev/null)\n", completeCommand)
	b.WriteString("  directive=$(printf '%s\\n' \"$out\" | tail -n 1)\n")
	b.WriteString("  candidates=$(printf '%s\\n' \"$out\" | sed '$d')\n\n")
	fmt.Fprint(&b, "  case \"$directive\" in\n")
	fmt.Fprint(&b, "    :default)\n")
	fmt.Fprint(&b, "      compopt -o default 2>/dev/null\n")
	fmt.Fprint(&b, "      ;;\n")
	fmt.Fprint(&b, "    :dirs)\n")
	fmt.Fprint(&b, "      compopt -o dirnames 2>/dev/null\n")
	fmt.Fprint(&b, "      ;;\n")
	fmt.Fprint(&b, "  esac\n\n")
	fmt.Fprint(&b, "  if [[ -n \"$candidates\" ]]; then\n")
	fmt.Fprint(&b, "    mapfile -t COMPREPLY < <(compgen -W \"$candidates\" -- \"$cur\")\n")
	fmt.Fprint(&b, "  fi\n")
	fmt.Fprint(&b, "  return 0\n")
	fmt.Fprint(&b, "}\n\n")
	fmt.Fprintf(&b, "complete -F _%s_complete %s\n", bin, strings.Join(uniqueStrings(completeNames), " "))

	return b.String()
}

func renderZsh(bin string, commands []topCommandInfo) string {
	var b strings.Builder
	fmt.Fprintf(&b, "#compdef %s\n", bin)
	fmt.Fprint(&b, "autoload -U +X bashcompinit && bashcompinit\n")
	fmt.Fprint(&b, renderBash(bin, commands))
	return b.String()
}

func renderFish(bin string, commands []topCommandInfo) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# fish completion for %s\n", bin)
	fmt.Fprintf(&b, "function __%s_complete\n", bin)
	fmt.Fprint(&b, "  set -l args (commandline -opc)\n")
	fmt.Fprint(&b, "  set -e args[1]\n")
	fmt.Fprint(&b, "  set -a args (commandline -ct)\n")
	fmt.Fprintf(&b, "  set -l out (%s __complete $args 2>/dev/null)\n", bin)
	fmt.Fprint(&b, "  set -l directive $out[-1]\n")
	b.WriteString("  printf '%s\\n' $out | string match -rv '^:'\n")
	fmt.Fprint(&b, "  switch $directive\n")
	fmt.Fprint(&b, "    case :default\n")
	fmt.Fprint(&b, "      __fish_complete_path (commandline -ct)\n")
	fmt.Fprint(&b, "    case :dirs\n")
	fmt.Fprint(&b, "      __fish_complete_directories (commandline -ct)\n")
	fmt.Fprint(&b, "  end\n")
	fmt.Fprint(&b, "end\n")
	fmt.Fprintf(&b, "complete -c %s -f -a '(__%s_complete)'\n", bin, bin)
	return b.String()
}

func topCommandNames(commands []topCommandInfo) []string {
	var names []string
	for _, cmd := range commands {
		names = append(names, cmd.Name)
		names = append(names, cmd.Aliases...)
	}
	return uniqueStrings(names)
}

func subcommandNames(commands []subcommandInfo) []string {
	var names []string
	for _, cmd := range commands {
		names = append(names, cmd.Name)
		names = append(names, cmd.Aliases...)
	}
	return uniqueStrings(names)
}

func collectFlagForms(flags []flagInfo) []string {
	var forms []string
	for _, flag := range flags {
		forms = append(forms, flag.DisplayForms...)
	}
	forms = append(forms, "--help", "-h")
	return uniqueStrings(forms)
}

func bashCaseLabel(names []string) string {
	names = uniqueStrings(names)
	if len(names) == 0 {
		return ""
	}
	return strings.Join(names, "|")
}

func fishSeenSubcommand(cmd interface{}) string {
	switch v := cmd.(type) {
	case topCommandInfo:
		return fmt.Sprintf("__fish_seen_subcommand_from %s", strings.Join(uniqueStrings(append([]string{v.Name}, v.Aliases...)), " "))
	case subcommandInfo:
		return fmt.Sprintf("__fish_seen_subcommand_from %s", strings.Join(uniqueStrings(append([]string{v.Name}, v.Aliases...)), " "))
	default:
		return "false"
	}
}

func fishFlagForms(flag flagInfo) []string {
	var out []string
	var longNames []string
	var shortNames []string
	for _, name := range flag.Names {
		if len(name) == 1 {
			shortNames = append(shortNames, name)
			continue
		}
		longNames = append(longNames, name)
	}

	longNames = uniqueStrings(longNames)
	shortNames = uniqueStrings(shortNames)

	if len(longNames) == 0 && len(shortNames) == 0 {
		return out
	}

	if len(longNames) > 0 && len(shortNames) > 0 {
		for _, longName := range longNames {
			for _, shortName := range shortNames {
				out = append(out, fmt.Sprintf("-l %s -s %s", longName, shortName))
			}
		}
		return out
	}

	for _, longName := range longNames {
		out = append(out, fmt.Sprintf("-l %s", longName))
	}
	for _, shortName := range shortNames {
		out = append(out, fmt.Sprintf("-s %s", shortName))
	}
	return out
}
