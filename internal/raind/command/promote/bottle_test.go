package promotecommand

import (
	"flag"
	"strings"
	"testing"

	"github.com/urfave/cli/v2"
)

func TestParsePromoteBottleCLIOptionsAcceptsTrailingFlags(t *testing.T) {
	ctx := newPromoteBottleTestContext(t, []string{
		"bottle.yaml",
		"--to", "resources",
		"-o", "manifests",
		"--namespace", "dev-ns",
		"--ingress-host", "app.raind.local",
	})

	opts, err := parsePromoteBottleCLIOptions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Path != "bottle.yaml" || opts.To != "resources" || opts.Output != "manifests" || opts.Namespace != "dev-ns" || opts.IngressHost != "app.raind.local" {
		t.Fatalf("unexpected parsed options: %#v", opts)
	}
}

func TestParsePromoteBottleCLIOptionsAcceptsEqualsTrailingFlags(t *testing.T) {
	ctx := newPromoteBottleTestContext(t, []string{
		"bottle.yaml",
		"--to=resources",
		"--output=out",
		"--namespace=prod",
		"--ingress-host=app.example.test",
	})

	opts, err := parsePromoteBottleCLIOptions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Output != "out" || opts.Namespace != "prod" || opts.IngressHost != "app.example.test" {
		t.Fatalf("unexpected parsed options: %#v", opts)
	}
}

func TestParsePromoteBottleCLIOptionsRejectsUnknownTrailingFlag(t *testing.T) {
	ctx := newPromoteBottleTestContext(t, []string{"bottle.yaml", "--unknown"})

	_, err := parsePromoteBottleCLIOptions(ctx)
	if err == nil || !strings.Contains(err.Error(), "unknown promote bottle option") {
		t.Fatalf("expected unknown trailing flag error, got %v", err)
	}
}

func TestParsePromoteBottleCLIOptionsRejectsMissingTrailingFlagValue(t *testing.T) {
	ctx := newPromoteBottleTestContext(t, []string{"bottle.yaml", "--ingress-host"})

	_, err := parsePromoteBottleCLIOptions(ctx)
	if err == nil || !strings.Contains(err.Error(), "--ingress-host requires a value") {
		t.Fatalf("expected missing value error, got %v", err)
	}
}

func newPromoteBottleTestContext(t *testing.T, args []string) *cli.Context {
	t.Helper()
	set := flag.NewFlagSet("promote-bottle", flag.ContinueOnError)
	set.String("to", "resources", "")
	set.String("output", "manifests", "")
	set.String("o", "manifests", "")
	set.String("namespace", "", "")
	set.String("ingress-host", "", "")
	if err := set.Parse(args); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	return cli.NewContext(cli.NewApp(), set, nil)
}
