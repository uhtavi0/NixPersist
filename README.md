# NixPersist — Linux persistence research toolkit written in Go!

> Educational, lab-friendly tooling for exploring triggerable and scheduled persistence on Linux.

**Educational use only.** This repository exists to support authorized testing, purple-team simulation, and research. Do not deploy NixPersist on systems without explicit permission from the owner. The authors and contributors take no responsibility for misuse.

## Project Scope
This is a personal project to learn Go Programming while exploring Linux Persistence Mechanisms that I find interesting. NixPersist provides a simple CLI to quickly install and remove any of the current selection of persistence mechanism for rapid Blue Team testing / Detection Engineering.

## Persistence Mechanisms

### 1. Rsylog Filters (Triggerable)
- Rsyslog filters can be configured to execute a program via the native "omprog" module when a log entry matches a given filter. This can be abused for any log that an attacker can control input for remotely. (*access.log, auth.log, etc)
-  For example: a filter can be created for `/var/log/auth.log` that launches a payload whenever the user `h4ck3r` attempts to establish an SSH connection.
- **Must be run as root**
-  `--check`, `--install`, `--remove` options are available to verify persistence method is possible, install, and remove. 
- `--apparmor` option can be used with either `--install` or `--remove` to enable or disable the apparmor profile for rsyslog.

### 2. Docker Compose (Boot / AutoStart)
- Launches a privileged container via docker-compose, mounts the host root filesystem, and executes a payload inside the container
- `--check`, `--install`, `--remove` for easy testing and cleanup
- Flags set the payload command (`-p`), container image (`-i`), service/container name (`-n`), and compose output directory (`-o`).
- Requires Docker with the current user running as root or part of the `docker` group.
