# KSS - Kubernetes pod status on steroid 💉

A beautiful and feature-rich tool to show the current status of a pod and its associated `containers` and `initContainers`.

This was developed out of frustration with `kubectl get pod` not showing much and `kubectl describe pod` showing way too much in a cryptic way. Debugging failed pods with a lot of `initContainers` and `sideCars` usually was done with `kubectl get pod -o yaml |less` with a lot of going up and down over a pager to figure out what's going on and a bunch of swearing 🔞. All those techniques for introspection and debugging are still useful and **KSS** is not planning to fully replace them but now thanks to it you can see quickly what happen and what fails and get your sanity back 😅.

<img width="1193" height="847" alt="image" src="https://github.com/user-attachments/assets/c65ac2e8-ba61-4fbe-a72f-f70af4f9814a" />

## Features ✨

- 🎨 **Beautiful UI** with color-coded status indicators, borders, and visual hierarchy
- 🔍 **Interactive pod selection** using [fzf](https://github.com/junegunn/fzf) with live preview
- ⏱️ **Watch mode** for real-time monitoring with auto-refresh
- 📊 **Detailed container information** including:
  - Container status with visual indicators (✓, ✗, ⏳)
  - Container age (how long it's been running)
  - Image names and versions
  - Restart counts
  - Ready status
- 📝 **Enhanced logging** with formatted output and separators
- 🏷️ **Labels and annotations** display with clean formatting
- 📅 **Events** with chronological sorting
- 🚨 **Better error detection** for common failure states

## Usage

### Basic Usage

You can specify a pod or multiple ones as argument to **KSS**, if you don't it will launch the lovely [fzf](https://github.com/junegunn/fzf) and let you choose the pod interactively, if there is only one pod available it will select it automatically. If you would like to choose multiple pods you can use the key [TAB] and select them, **KSS** will then show them all.

**KSS** shows a preview when running with fzf, it will try to do the preview with itself if it cannot find itself in the `PATH` it will fallback to a good ol' and boring `kubectl describe` 👴🏼👵🏻.

### Command Line Options

```
Usage: kss [OPTIONS] [POD...]

Options:
  -n, --namespace NAMESPACE    Use namespace
  -r, --restrict REGEXP        Restrict to show only those containers (regexp)
  -l, --showlog                Show logs of containers
  --maxlines INT               Maximum line when showing logs (default: -1)
  -L, --labels                 Show labels
  -A, --annotations            Show annotations
  -E, --events                 Show events
  -w, --watch                  Watch mode (auto-refresh)
  --watch-interval SECONDS     Watch refresh interval in seconds (default: 2)
  -h, --help                   Display help message
```

### Examples

#### Interactive Pod Selection

```bash
# Launch fzf to interactively select a pod
kss

# Select from a specific namespace
kss -n production

# Select multiple pods (use TAB in fzf)
kss
```

#### Direct Pod Selection

```bash
# Show status for a specific pod
kss my-pod

# Show status for multiple pods
kss pod-1 pod-2 pod-3

# Show status with namespace
kss -n production my-pod
```

#### Viewing Logs

```bash
# Show logs for all containers
kss my-pod -l

# Show last 50 lines of logs
kss my-pod -l --maxlines 50

# Show logs only for containers matching a regex
kss my-pod -r "app" -l

# Show logs for init containers only
kss my-pod -r "init" -l
```

#### Watch Mode

```bash
# Watch a pod with default 2-second refresh
kss my-pod -w

# Watch with custom refresh interval (5 seconds)
kss my-pod -w --watch-interval 5

# Watch multiple pods
kss pod-1 pod-2 -w

# Watch with logs
kss my-pod -w -l
```

#### Labels and Annotations

```bash
# Show labels
kss my-pod -L

# Show annotations
kss my-pod -A

# Show both
kss my-pod -L -A
```

#### Events

```bash
# Show events for a pod
kss my-pod -E

# Show events with other information
kss my-pod -E -L
```

#### Combined Usage

```bash
# Full information with logs, labels, and events
kss my-pod -l -L -A -E

# Watch mode with logs and events
kss my-pod -w -l -E --watch-interval 3
```

## Output Format

### Pod Header

The pod information is displayed in a beautiful box with:

- Pod name
- Overall status (✅ SUCCESS, 🔄 RUNNING, ❌ FAIL)
- Namespace
- Phase
- Age (time since pod creation)

### Container Information

For each container, KSS displays:

- **Status**: Visual indicator (✓, ✗, ⏳) with color coding
  - ✓ Green: Success/Running
  - ✗ Red: Failed/Error
  - ⏳ Yellow: Waiting
- **Image**: Container image name
- **Restarts**: Number of restarts (highlighted if > 0)
- **Age**: How long the container has been in its current state
- **Ready**: Whether the container is ready

### Status Colors

- 🟢 **Green**: Success, Running, Ready
- 🔵 **Blue**: Running, Active
- 🟡 **Yellow**: Waiting, Warning, Restarts
- 🔴 **Red**: Failed, Error, Not Ready

## Install

### Packages

#### Homebrew

You can install **KSS** latest with homebrew, you just have to fire up those
commands in your shell and **KSS** and its zsh completions will be installed :

```shell
brew tap chmouel/kss https://github.com/chmouel/kss
brew install kss
```

This has been tested as working on [linuxbrew](https://docs.brew.sh/Homebrew-on-Linux) too.

#### Arch

It's available on Arch AUR [here](https://aur.archlinux.org/packages/kss).

Install it with your favourite aur installer (i.e: [yay](https://github.com/Jguer/yay))

```bash
yay -S kss
```

### Manual install

You just make sure you have [Go](https://golang.org/) (>=1.21), [fzf](https://github.com/junegunn/fzf) and kubectl. You can build from source:

```shell
git clone https://github.com/chmouel/kss
cd kss
go build -o kss main.go
sudo cp kss /usr/local/bin/
```

Or checkout this GIT repo and build the binary into your path.

With zsh you can install the [_kss](./_kss) completionfile  to your [fpath](https://unix.stackexchange.com/a/33898).

### Requirements

- **kubectl**: Must be installed and configured
- **fzf**: Required for interactive pod selection (install via your package manager)
- **Go**: Only needed if building from source (>=1.21)


## Tips & Tricks 💡

### Quick Debugging Workflow

```bash
# 1. Find and select the problematic pod
kss

# 2. View logs for the failing container
kss my-pod -r "app" -l --maxlines 100

# 3. Check events to see what happened
kss my-pod -E

# 4. Watch the pod recover
kss my-pod -w
```

### Using with kubectl aliases

```bash
# Add to your .bashrc or .zshrc
alias kp='kss'
alias kpw='kss -w'
alias kpl='kss -l'
```

### Filtering containers

```bash
# Show only sidecar containers
kss my-pod -r "sidecar"

# Show only init containers
kss my-pod -r "init"

# Show containers matching multiple patterns (use regex)
kss my-pod -r "app|worker"
```

## Troubleshooting

### fzf not found

If you get an error about fzf not being found, install it:

```bash
# macOS
brew install fzf

# Linux
sudo apt install fzf  # Debian/Ubuntu
sudo yum install fzf  # RHEL/CentOS
```

### kubectl not configured

Make sure kubectl is installed and your kubeconfig is set up:

```bash
kubectl get pods
```

### No pods found

If KSS says "No pods is no news which is arguably no worries", it means:

- No pods exist in the current/default namespace
- You might need to specify a namespace with `-n`
- Check your kubectl context: `kubectl config current-context`

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for details.

## Misc

- The code has been rewritten in Go for better performance and easier distribution. The original Python version was getting quite tortured, like some sort of spaghetti plate 🍝 with greasy meatballs 🥩 on the top, the kind of stuff you start to write quickly and dirty out of frustration to fix a problem and it grows until it really become an unreadable beast. So we rewrote it in Go! 🎉

The Go version maintains all the functionality of the Python version while being faster and easier to distribute as a single binary. It also includes many new features like watch mode, better UI, and enhanced container information display.

I may do a [krew](https://github.com/kubernetes-sigs/krew) plugin if this get [requested](https://github.com/chmouel/kss/issues/1) enough. Watch this space as cool people would say 😎🏄🤙.
