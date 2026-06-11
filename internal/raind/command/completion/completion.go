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
		Action: func(ctx *cli.Context) error {
			shell := strings.ToLower(strings.TrimSpace(ctx.Args().First()))
			if shell == "" {
				return fmt.Errorf("shell is required: bash|zsh|fish")
			}

			info := collectInfo(app)
			switch shell {
			case "bash":
				fmt.Fprint(ctx.App.Writer, renderBash(app.Name, info))
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
	var b strings.Builder
	fmt.Fprintf(&b, "# bash completion for %s\n", bin)
	fmt.Fprintf(&b, "_%s_complete() {\n", bin)
	fmt.Fprint(&b, "  local cur prev\n")
	fmt.Fprint(&b, "  COMPREPLY=()\n")
	fmt.Fprint(&b, "  cur=\"${COMP_WORDS[COMP_CWORD]}\"\n")
	fmt.Fprint(&b, "  prev=\"${COMP_WORDS[COMP_CWORD-1]}\"\n\n")

	fmt.Fprintf(&b, "  local top_commands=\"%s\"\n\n", strings.Join(topCommandNames(commands), " "))

	fmt.Fprint(&b, "  if [[ $COMP_CWORD -eq 1 ]]; then\n")
	fmt.Fprint(&b, "    COMPREPLY=( $(compgen -W \"${top_commands}\" -- \"${cur}\") )\n")
	fmt.Fprint(&b, "    return 0\n")
	fmt.Fprint(&b, "  fi\n\n")

	fmt.Fprint(&b, "  case \"${COMP_WORDS[1]}\" in\n")
	for _, top := range commands {
		caseLabel := bashCaseLabel(append([]string{top.Name}, top.Aliases...))
		fmt.Fprintf(&b, "    %s)\n", caseLabel)
		if len(top.Subcommands) > 0 {
			fmt.Fprintf(&b, "      local subcommands=\"%s\"\n", strings.Join(subcommandNames(top.Subcommands), " "))
			fmt.Fprint(&b, "      if [[ $COMP_CWORD -eq 2 ]]; then\n")
			fmt.Fprint(&b, "        COMPREPLY=( $(compgen -W \"${subcommands}\" -- \"${cur}\") )\n")
			fmt.Fprint(&b, "        return 0\n")
			fmt.Fprint(&b, "      fi\n\n")
			fmt.Fprint(&b, "      case \"${COMP_WORDS[2]}\" in\n")
			for _, sub := range top.Subcommands {
				subCase := bashCaseLabel(append([]string{sub.Name}, sub.Aliases...))
				flags := collectFlagForms(sub.Flags)
				fmt.Fprintf(&b, "        %s)\n", subCase)
				if len(sub.Subcommands) > 0 {
					fmt.Fprintf(&b, "          local subsubcommands=\"%s\"\n", strings.Join(subcommandNames(sub.Subcommands), " "))
					fmt.Fprint(&b, "          if [[ $COMP_CWORD -eq 3 ]]; then\n")
					fmt.Fprint(&b, "            COMPREPLY=( $(compgen -W \"${subsubcommands}\" -- \"${cur}\") )\n")
					fmt.Fprint(&b, "            return 0\n")
					fmt.Fprint(&b, "          fi\n\n")
					fmt.Fprint(&b, "          case \"${COMP_WORDS[3]}\" in\n")
					for _, subsub := range sub.Subcommands {
						subsubCase := bashCaseLabel(append([]string{subsub.Name}, subsub.Aliases...))
						subsubFlags := collectFlagForms(subsub.Flags)
						if len(subsubFlags) == 0 {
							continue
						}
						fmt.Fprintf(&b, "            %s)\n", subsubCase)
						fmt.Fprintf(&b, "              COMPREPLY=( $(compgen -W \"%s\" -- \"${cur}\") )\n", strings.Join(subsubFlags, " "))
						fmt.Fprint(&b, "              return 0\n")
						fmt.Fprint(&b, "              ;;\n")
					}
					fmt.Fprint(&b, "          esac\n")
				} else if len(flags) > 0 {
					fmt.Fprintf(&b, "          COMPREPLY=( $(compgen -W \"%s\" -- \"${cur}\") )\n", strings.Join(flags, " "))
					fmt.Fprint(&b, "          return 0\n")
				}
				fmt.Fprint(&b, "          ;;\n")
			}
			fmt.Fprint(&b, "      esac\n")
		}
		fmt.Fprint(&b, "      ;;\n")
	}
	fmt.Fprint(&b, "  esac\n\n")

	fmt.Fprint(&b, "  return 0\n")
	fmt.Fprint(&b, "}\n\n")
	fmt.Fprintf(&b, "complete -F _%s_complete %s\n", bin, bin)

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

	topNames := strings.Join(topCommandNames(commands), " ")
	fmt.Fprintf(&b, "complete -c %s -n 'test (count (commandline -opc)) -eq 1' -a '%s'\n", bin, topNames)

	for _, top := range commands {
		topMatch := fishSeenSubcommand(top)
		if len(top.Subcommands) > 0 {
			subNames := strings.Join(subcommandNames(top.Subcommands), " ")
			fmt.Fprintf(&b, "complete -c %s -n '%s; and test (count (commandline -opc)) -eq 2' -a '%s'\n", bin, topMatch, subNames)
		}
		for _, sub := range top.Subcommands {
			subMatch := fishSeenSubcommand(sub)
			if len(sub.Subcommands) > 0 {
				subSubNames := strings.Join(subcommandNames(sub.Subcommands), " ")
				fmt.Fprintf(&b, "complete -c %s -n '%s; and %s; and test (count (commandline -opc)) -eq 3' -a '%s'\n", bin, topMatch, subMatch, subSubNames)
			}
			flags := sub.Flags
			if len(flags) == 0 {
				// still allow sub-sub flags below
			} else {
				for _, flag := range flags {
					flagForms := fishFlagForms(flag)
					for _, form := range flagForms {
						requiresArg := ""
						if flag.TakesValue {
							requiresArg = " -r"
						}
						fmt.Fprintf(&b, "complete -c %s -n '%s; and %s' %s%s\n", bin, topMatch, subMatch, form, requiresArg)
					}
				}
			}

			for _, subsub := range sub.Subcommands {
				subSubMatch := fishSeenSubcommand(subsub)
				if len(subsub.Flags) == 0 {
					continue
				}
				for _, flag := range subsub.Flags {
					flagForms := fishFlagForms(flag)
					for _, form := range flagForms {
						requiresArg := ""
						if flag.TakesValue {
							requiresArg = " -r"
						}
						fmt.Fprintf(&b, "complete -c %s -n '%s; and %s; and %s' %s%s\n", bin, topMatch, subMatch, subSubMatch, form, requiresArg)
					}
				}
			}
		}
	}

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
