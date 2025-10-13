# NixPersist — Linux persistence research toolkit written in Go!

> Educational, lab-friendly tooling for exploring triggerable and scheduled persistence on Linux.

**Educational use only.** This repository exists to support authorized testing, purple-team simulation, and research. Do not deploy NixPersist on systems without explicit permission from the owner. The authors and contributors take no responsibility for misuse.

## Project Scope
This is a personal project to learn Go Programming while exploring Linux Persistence Mechanisms that I find interesting. NixPersist provides a simple CLI to quickly install and remove any of the current selection of persistence mechanism for rapid Blue Team testing / Detection Engineering.

## Persistence Mechanisms

### 1. Rsyslog Filters (Triggerable)
NixPersist exposes two rsyslog triggerable execution methods - Shell Execute and the Module OMPROG. Based on this [PoC](https://gist.github.com/0xshaft03/a5dc1f4da395c37f9a130a0f5583b575) by 0xshaft03.

- `rsyslog` flag (shell execute): appends a single-line trigger to `/etc/rsyslog.conf` that executes the provided payload when the specified substring is observed in any of rsyslog's logging facilities. It is quick to deploy, easy to inspect, and mirrors the “one-liner” PoC from the notes. `--output` lets you render elsewhere, while `--install`/`--remove` manage the live config and reload rsyslog.
- `rsyslog-omprog` flag (imfile + omprog): installs an additional conf under `/etc/rsyslog.d/99-nixpersist.conf`. It can read arbitrary files via `imfile`, isolates logic in a dedicated ruleset, and executes the payload via `omprog`. Use `--log-file-in`, `--payload`, and `--trigger` to match the PoC defaults.
- Both modules support `--check`, `--install`, and `--remove`, plus `--apparmor` to disable the rsyslog profile during install and re-enable it on removal when needed. **Must be run as root.**

### 2. Docker Compose (Boot / AutoStart)
- Launches a privileged container via docker-compose, mounts the host's root filesystem, and executes a payload inside the container
- `--check`, `--install`, `--remove` for easy testing and cleanup
- Flags set the payload command (`-p`), container image (`-i`), service/container name (`-n`), and compose output directory (`-o`).
- Requires Docker with the current user running as root or part of the `docker` group.
