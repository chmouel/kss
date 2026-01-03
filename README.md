# KSS - Enhanced Kubernetes Pod Inspection

I present to you KSS, a refined utility designed to illuminate the current status of a Kubernetes pod and its associated containers and initContainers with clarity and precision.

The standard `kubectl get pod` command, while functional, occasionally lacks the immediate depth one requires. Conversely, `kubectl describe pod` can offer an overwhelming abundance of detail that may obscure the pertinent facts. KSS bridges this gap, offering a comprehensive, aesthetically pleasing, and digestible overview of your pod's health, thereby allowing you to diagnose issues with dignity and efficiency.

<img width="1193" height="847" alt="image" src="https://github.com/user-attachments/assets/c65ac2e8-ba61-4fbe-a72f-f70af4f9814a" />

## Distinguishing Features

- **Refined User Interface**: Presents data with elegant color-coding, borders, and a clear visual hierarchy.
- **Interactive Selection**: Utilizes [fzf](https://github.com/junegunn/fzf) to provide a sophisticated interactive search with live previews.
- **Continuous Monitoring**: Offers a "Watch mode" to observe the real-time status of your deployments.
- **Doctor Analysis**: Heuristic diagnosis for common failure modes, with log pattern detection to hasten triage.
- **AI Explanations (Optional)**: When enabled, it consults Gemini to provide a concise diagnosis and a suggested remedy.
- **Comprehensive Container Details**:
  - Visual status indicators (Success, Failure, Waiting).
  - Accurate age and duration metrics.
  - Image specifications.
  - Restart counters for identifying instability.
  - Readiness checks.
- **Enhanced Logging**: Displays logs with proper formatting and distinct separators for clarity.
- **Metadata Inspection**: elegantly presents Labels and Annotations.
- **Richer Pod Context**: Surfaces Namespace, Phase, Age, Pod IP, Node, QoS, ServiceAccount, Priority, and Conditions.
- **Chronological Events**: Lists pod events sorted by time to aid in forensic analysis.
- **Intelligent Error Detection**: Highlights common failure states such as `CrashLoopBackOff` or `ImagePullBackOff`.

## Instructions for Use

### Basic Operation

One may specify a pod—or indeed, multiple pods—as arguments to the KSS command. Should you decline to provide an argument, the application will graciously launch the interactive selector (via `fzf`), allowing you to choose your desired pod from the list. If only a single pod is present, KSS will select it automatically for your convenience.

To select multiple pods in the interactive view, one needs only to press the `TAB` key. KSS will then proceed to display the details for all selected items.

Please note that the interactive preview attempts to utilize KSS itself to render the information. Should KSS not be found within your system's `PATH`, it will resort to the standard `kubectl describe` command.

### Command Line Options

```text
Usage: kss [OPTIONS] [POD...]

Options:
  -n, --namespace NAMESPACE    Specify the namespace to inspect.
  -r, --restrict REGEXP        Restrict the display to containers matching the provided regular expression.
  -l, --showlog                Retrieve and display container logs.
  --maxlines INT               Limit the number of log lines displayed (default: all lines).
  -L, --labels                 Reveal the pod's labels.
  -A, --annotations            Reveal the pod's annotations.
  -E, --events                 List the pod's events.
  -w, --watch                  Enable watch mode for continuous monitoring.
  --watch-interval SECONDS     Set the refresh interval for watch mode (default: 2 seconds).
  -d, --doctor                 Enable heuristic analysis (Doctor mode).
  -s, --shell                  Open an interactive shell in the selected pod.
  --explain                    Enable AI explanation for pod failures.
  --model MODEL                AI model to use (default: gemini-2.5-flash-lite).
  -p, --persona PERSONA        AI persona: butler, sergeant, hacker, pirate, genz (default: random).
  --completion SHELL           Output shell completion code (bash, zsh).
  -h, --help                   Display the help message.
```

### Examples

#### Interactive Selection

```bash
# Launch the interactive selector
kss

# browse pods within a specific namespace
kss -n production
```

#### Direct Selection

```bash
# Inspect a specific pod
kss my-pod

# Inspect multiple pods simultaneously
kss pod-1 pod-2 pod-3

# Inspect a pod within a specific namespace
kss -n production my-pod
```

#### Shell Access

```bash
# Open a shell in a selected pod
kss -s

# Jump straight into a pod's shell
kss -s my-pod
```

If the pod hosts multiple containers, KSS will courteously ask which one you wish to enter.

#### Log Inspection

```bash
# Display logs for all containers
kss my-pod -l

# Display the final 50 lines of logs
kss my-pod -l --maxlines 50

# Display logs only for containers matching a pattern
kss my-pod -r "app" -l
```

#### Continuous Monitoring

```bash
# Monitor a pod (refreshes every 2 seconds by default)
kss my-pod -w

# Monitor with a custom refresh interval of 5 seconds
kss my-pod -w --watch-interval 5
```

#### Doctor Analysis

```bash
# Enable heuristics even if the pod looks healthy
kss my-pod -d
```

Doctor analysis also runs automatically when a container enters a failed state.

#### AI Explanation

```bash
# Request Gemini's explanation (requires GEMINI_API_KEY)
export GEMINI_API_KEY=your-token
kss my-pod --explain

# Select a persona and model
kss my-pod --explain -p hacker --model gemini-2.5-flash
```

If you prefer a neutral, technical tone, the `hacker` persona is the best suited of the lot.

Personas at a glance:

| Persona | Tone |
| --- | --- |
| butler | Polite, formal, efficient (British butler) |
| sergeant | Direct, demanding, no-nonsense |
| hacker | Neutral, technical, concise |
| pirate | Rough, nautical, a touch playful |
| genz | Casual, slangy, emoji-friendly |

#### Metadata & Events

```bash
# Display labels and annotations
kss my-pod -L -A

# Display the sequence of events
kss my-pod -E

# A comprehensive view including logs, metadata, and events
kss my-pod -l -L -A -E
```

## Installation

### Package Managers

#### Homebrew (macOS & Linux)

One may install the latest version of KSS via Homebrew. Simply execute the following commands in your terminal:

```shell
brew tap chmouel/kss https://github.com/chmouel/kss
brew install kss
```

#### Arch Linux

For users of Arch Linux, the package is available on the AUR [here](https://aur.archlinux.org/packages/kss). You may install it using your preferred helper, such as `yay`:

```bash
yay -S kss
```

### Manual Installation

Ensure that you have [Go](https://golang.org/) (version 1.21 or higher), [fzf](https://github.com/junegunn/fzf), and `kubectl` installed on your system. You may then build the application from the source:

```shell
git clone https://github.com/chmouel/kss
cd kss
go build -o kss main.go
sudo cp kss /usr/local/bin/
```

### Shell Completion

KSS provides dynamic completion for Bash and Zsh.

**Bash:**
```bash
source <(kss --completion bash)
```

**Zsh:**
```zsh
source <(kss --completion zsh)
```

You can also generate the scripts and save them to your completion directory:
```bash
kss --completion bash > /etc/bash_completion.d/kss
# or for zsh
kss --completion zsh > /usr/share/zsh/site-functions/_kss
```

Completion suggestions include namespaces, pods, and persona names for your convenience.

### Prerequisites

- **kubectl**: Must be installed and properly configured to communicate with your cluster.
- **fzf**: Essential for the interactive selection feature.
- **Go**: Required only if you intend to compile the application from source.
- **GEMINI_API_KEY**: Required only if you wish to use `--explain` for AI-assisted diagnosis.

## Recommendations & Troubleshooting

### A Suggested Workflow

1.  **Identify**: Run `kss` to interactively locate the pod in distress.
2.  **Investigate**: Use `kss my-pod -r "app" -l --maxlines 100` to examine the recent logs of the primary container.
3.  **Contextualize**: Execute `kss my-pod -E` to review recent cluster events associated with the pod.
4.  **Observe**: Finally, employ `kss my-pod -w` to monitor the pod as it attempts to recover.

### Aliases

To expedite your workflow, you might consider adding the following aliases to your shell configuration:

```bash
alias kp='kss'
alias kpw='kss -w'
alias kpl='kss -l'
```

### Common Issues

**fzf is missing:**
If the application reports that `fzf` cannot be found, please ensure it is installed via your system's package manager (e.g., `brew install fzf` or `sudo apt install fzf`).

**No pods found:**
Should the application report that "No pods is no news which is arguably no worries" (a whimsical way of stating that the list is empty), it typically indicates:
- There are indeed no pods in the current namespace.
- You may need to specify the correct namespace using the `-n` flag.
- Your `kubectl` context may need adjustment.

## Screenshots

### Events and Error Display
<img width="1612" height="894" alt="image" src="https://github.com/user-attachments/assets/ccbead7a-c1ad-4b0a-a3ae-aa22422a1731" />

## Contributing

Your contributions are most welcome. Should you wish to improve this tool, please feel free to submit a Pull Request.

## License

This software is licensed under the Apache License, Version 2.0. Please refer to the [LICENSE](LICENSE) file for further details.

## Remarks

The application has been rewritten in Go to ensure superior performance and ease of distribution. The previous Python iteration, while valiant, had become somewhat unwieldy. The new Go implementation maintains all prior functionality—and indeed, expands upon it—while remaining a single, efficient binary.

I am considering the creation of a [krew](https://github.com/kubernetes-sigs/krew) plugin, should there be sufficient interest from the community.
