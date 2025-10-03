package dockercompose

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strings"
)

// Result captures discovery data about the local Docker installation.
type Result struct {
	DockerAvailable   bool
	ComposeAvailable  bool
	UserIsRoot        bool
	UserInDockerGroup bool
	DockerPsSucceeded bool
	Images            []string
	Containers        []string
	Notes             []string
}

// HasAccess reports whether the current user is likely able to interact with Docker.
func (r Result) HasAccess() bool {
	return r.UserIsRoot || r.UserInDockerGroup || r.DockerPsSucceeded
}

// Render returns a human-readable summary of the diagnostic results.
func (r Result) Render() string {
	b := &strings.Builder{}
	writeLine := func(label string, ok bool) {
		status := "NO"
		if ok {
			status = "YES"
		}
		fmt.Fprintf(b, "- %s: %s\n", label, status)
	}
	writeLine("docker binary present", r.DockerAvailable)
	writeLine("docker compose available", r.ComposeAvailable)
	writeLine("user has docker access", r.HasAccess())

	if len(r.Images) > 0 {
		b.WriteString("\nImages:\n")
		for _, img := range r.Images {
			fmt.Fprintf(b, "- %s\n", img)
		}
	}

	if len(r.Containers) > 0 {
		if len(r.Images) == 0 {
			b.WriteString("\n")
		}
		b.WriteString("Containers:\n")
		for _, c := range r.Containers {
			fmt.Fprintf(b, "- %s\n", c)
		}
	}

	if len(r.Notes) > 0 {
		if len(r.Images) == 0 && len(r.Containers) == 0 {
			b.WriteString("\n")
		}
		b.WriteString("Notes:\n")
		for _, n := range r.Notes {
			fmt.Fprintf(b, "- %s\n", n)
		}
	}

	return b.String()
}

// Check inspects the local system for Docker requirements.
func Check() Result {
	var r Result

	r.DockerAvailable = hasCommand("docker")
	r.ComposeAvailable = hasCompose()
	r.UserIsRoot = os.Geteuid() == 0
	if r.UserIsRoot {
		r.Notes = append(r.Notes, "running as root")
	}

	if !r.UserIsRoot {
		r.UserInDockerGroup = userInGroup("docker", &r)
	}

	if r.DockerAvailable {
		r.DockerPsSucceeded = tryDockerPs(&r)
		if imgs := listDockerImages(&r); len(imgs) > 0 {
			r.Images = imgs
		}
		if ctrs := listDockerContainers(&r); len(ctrs) > 0 {
			r.Containers = ctrs
		}
	}

	return r
}

func hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func hasCompose() bool {
	if _, err := exec.LookPath("docker"); err == nil {
		cmd := commandRunner("docker", "compose", "version")
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		if err := cmd.Run(); err == nil {
			return true
		}
	}
	if _, err := exec.LookPath("docker-compose"); err == nil {
		return true
	}
	return false
}

func userInGroup(target string, r *Result) bool {
	u, err := user.Current()
	if err != nil {
		r.Notes = append(r.Notes, fmt.Sprintf("user lookup failed: %v", err))
		return false
	}
	gids, err := u.GroupIds()
	if err != nil {
		r.Notes = append(r.Notes, fmt.Sprintf("group enumeration failed: %v", err))
		return false
	}
	for _, gid := range gids {
		grp, err := user.LookupGroupId(gid)
		if err != nil {
			continue
		}
		if grp.Name == target {
			r.Notes = append(r.Notes, "current user is a member of the docker group")
			return true
		}
	}
	return false
}

func tryDockerPs(r *Result) bool {
	cmd := commandRunner("docker", "ps")
	if output, err := cmd.CombinedOutput(); err == nil {
		if len(strings.TrimSpace(string(output))) > 0 {
			r.Notes = append(r.Notes, "docker ps returned data")
		}
		return true
	}
	r.Notes = append(r.Notes, "docker ps failed (user may lack permissions or daemon stopped)")
	return false
}

func listDockerImages(r *Result) []string {
	cmd := commandRunner("docker", "image", "ls", "--format", "{{.Repository}}:{{.Tag}} ({{.ID}})")
	output, err := cmd.CombinedOutput()
	if err != nil {
		r.Notes = append(r.Notes, fmt.Sprintf("docker image ls failed: %v", err))
		return nil
	}
	return nonEmptyLines(string(output))
}

func listDockerContainers(r *Result) []string {
	cmd := commandRunner("docker", "ps", "-a", "--format", "{{.Names}} ({{.Image}}) status {{.Status}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		r.Notes = append(r.Notes, fmt.Sprintf("docker ps -a failed: %v", err))
		return nil
	}
	return nonEmptyLines(string(output))
}

func nonEmptyLines(s string) []string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	var out []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}
