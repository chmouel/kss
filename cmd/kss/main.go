// Package main implements KSS - Enhanced Kubernetes Pod Inspection.
// A beautiful and feature-rich tool to show the current status of pods
// and their associated containers and initContainers.
package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/chmouel/kss/internal/kube"
	"github.com/chmouel/kss/internal/model"
	"github.com/chmouel/kss/internal/ui"
	"github.com/chmouel/kss/internal/util"
)

// clearScreen clears the terminal screen (used in watch mode)
func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func main() {
	args := parseArgs()

	if args.Completion != "" {
		printCompletion(args.Completion)
		os.Exit(0)
	}

	for i, pod := range args.Pods {
		args.Pods[i] = util.ResourceName(pod)
	}

	kctl := "kubectl"

	if args.Namespace != "" {
		kctl = fmt.Sprintf("kubectl -n %s", args.Namespace)
	}
	kubectlBaseArgs := kube.KubectlArgs(args)

	myself, err := os.Executable()
	if err != nil || myself == "" {
		myself = util.Which("kss")
	}
	preview := fmt.Sprintf("%s describe {}", kctl)
	if myself != "" {
		// Use preview mode for fzf - compact and clean
		preview = myself + " --preview"
		if args.Namespace != "" {
			preview += fmt.Sprintf(" -n %s", args.Namespace)
		}
		if args.Annotations {
			preview += " -A"
		}
		if args.Labels {
			preview += " -L"
		}
		preview += " {}"
	}

	if args.Shell {
		pod := ""
		if len(args.Pods) > 0 {
			pod = args.Pods[0]
		} else {
			queryArgs := ""
			if len(args.Pods) > 0 {
				queryArgs = fmt.Sprintf("-q '%s'", strings.Join(args.Pods, " "))
			}
			runcmd := fmt.Sprintf("%s get pods -o name|fzf -0 -n 1 -m -1 %s --preview-window 'right:60%%:wrap' --preview='%s'", kctl, queryArgs, preview)
			cmd := exec.Command("sh", "-c", runcmd)
			output, err := cmd.Output()
			if err != nil {
				fmt.Println("No pods is no news which is arguably no worries.")
				os.Exit(1)
			}
			podNames := strings.TrimSpace(string(output))
			if podNames == "" {
				fmt.Println("No pods is no news which is arguably no worries.")
				os.Exit(1)
			}
			pods := strings.Split(strings.ReplaceAll(podNames, "pod/", ""), "\n")
			pod = strings.TrimSpace(pods[0])
		}

		if pod == "" {
			fmt.Println("No pods is no news which is arguably no worries.")
			os.Exit(1)
		}

		if err := kube.RunShell(args, kubectlBaseArgs, pod); err != nil {
			fmt.Printf("Unable to start shell in pod %s: %v\n", pod, err)
			os.Exit(1)
		}
		return
	}

	queryArgs := ""
	if len(args.Pods) > 0 {
		queryArgs = fmt.Sprintf("-q '%s'", strings.Join(args.Pods, " "))
	}

	runcmd := fmt.Sprintf("%s get pods -o name|fzf -0 -n 1 -m -1 %s --preview-window 'right:60%%:wrap' --preview='%s'", kctl, queryArgs, preview)

	cmd := exec.Command("sh", "-c", runcmd)
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("No pods is no news which is arguably no worries.")
		os.Exit(1)
	}

	podNames := strings.TrimSpace(string(output))
	if podNames == "" {
		fmt.Println("No pods is no news which is arguably no worries.")
		os.Exit(1)
	}

	pods := strings.Split(strings.ReplaceAll(podNames, "pod/", ""), "\n")

	// Handle watch mode
	if args.Watch {
		interval := time.Duration(args.WatchInterval) * time.Second
		if interval == 0 {
			interval = 2 * time.Second
		}

		// Handle Ctrl+C gracefully
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		for {
			select {
			case <-sigChan:
				fmt.Println("\nExiting watch mode...")
				os.Exit(0)
			default:
				clearScreen()
				fmt.Printf("%s Watching pods (refresh every %v, press Ctrl+C to exit)\n\n",
					util.ColorText("⏱", "cyan"), interval)

				for _, pod := range pods {
					pod = strings.TrimSpace(pod)
					if pod == "" {
						continue
					}

					cmdline := fmt.Sprintf("%s get pod %s -ojson", kctl, pod)
					cmd := exec.Command("sh", "-c", cmdline)
					output, err := cmd.CombinedOutput()
					if err != nil {
						fmt.Printf("Error running '%s'\n", cmdline)
						continue
					}

					var podObj model.Pod
					if err := json.Unmarshal(output, &podObj); err != nil {
						fmt.Printf("Error parsing JSON: %v\n", err)
						continue
					}

					ui.PrintPodInfo(podObj, kctl, pod, args)
				}

				time.Sleep(interval)
			}
		}
	}

	// Normal mode
	for _, pod := range pods {
		pod = strings.TrimSpace(pod)
		if pod == "" {
			continue
		}

		cmdline := fmt.Sprintf("%s get pod %s -ojson", kctl, pod)
		cmd := exec.Command("sh", "-c", cmdline)
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("There was some problem running '%s'\n", cmdline)
			os.Exit(1)
		}

		var podObj model.Pod
		if err := json.Unmarshal(output, &podObj); err != nil {
			fmt.Printf("Error parsing JSON: %v\n", err)
			os.Exit(1)
		}

		ui.PrintPodInfo(podObj, kctl, pod, args)

		if len(pods) > 1 {
			fmt.Println()
		}
	}
}

// parseArgs parses command-line arguments and returns an Args struct
func parseArgs() model.Args {
	args := model.Args{
		MaxLines:      "-1",
		WatchInterval: 2,
		Model:         "gemini-2.5-flash-lite",
	}

	personas := []string{"neutral", "butler", "sergeant", "hacker", "pirate", "genz"}
	if envPersona := os.Getenv("KSS_PERSONA"); envPersona != "" {
		args.Persona = envPersona
	}

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "-n", "--namespace":
			if i+1 < len(os.Args) {
				args.Namespace = os.Args[i+1]
				i++
			}
		case "-r", "--restrict":
			if i+1 < len(os.Args) {
				args.Restrict = os.Args[i+1]
				i++
			}
		case "-l", "--showlog":
			args.ShowLog = true
		case "--maxlines":
			if i+1 < len(os.Args) {
				args.MaxLines = os.Args[i+1]
				i++
			}
		case "-L", "--labels":
			args.Labels = true
		case "-A", "--annotations":
			args.Annotations = true
		case "-E", "--events":
			args.Events = true
		case "-w", "--watch":
			args.Watch = true
		case "--watch-interval":
			if i+1 < len(os.Args) {
				var interval int
				_, _ = fmt.Sscanf(os.Args[i+1], "%d", &interval)
				args.WatchInterval = interval
				i++
			}
		case "--preview":
			args.Preview = true
		case "-d", "--doctor":
			args.Doctor = true
		case "-s", "--shell":
			args.Shell = true
		case "--explain":
			args.Explain = true
		case "--model":
			if i+1 < len(os.Args) {
				args.Model = os.Args[i+1]
				i++
			}
		case "-p", "--persona":
			if i+1 < len(os.Args) {
				args.Persona = os.Args[i+1]
				i++
			}
		case "--completion":
			if i+1 < len(os.Args) {
				args.Completion = os.Args[i+1]
				i++
			}
		case "-h", "--help":
			printHelp()
			os.Exit(0)
		default:
			if !strings.HasPrefix(arg, "-") {
				args.Pods = append(args.Pods, arg)
			}
		}
	}

	// Pick a random persona if not specified
	if args.Persona == "" {
		// #nosec G404
		args.Persona = personas[rand.Intn(len(personas))]
	}

	return args
}

// printHelp displays the help message with usage examples
func printHelp() {
	helpText := `
╔════════════════════════════════════════════════════════════════════════╗
║                    KSS - Kubernetes Pod Status                         ║
║                 Enhanced Kubernetes Pod Inspection                     ║
╚════════════════════════════════════════════════════════════════════════╝

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
  -d, --doctor                 Enable heuristic analysis (Doctor mode)
  -s, --shell                  Open an interactive shell in the selected pod
  --explain                    Enable AI explanation for pod failures
  --model MODEL                AI Model to use (default: gemini-2.5-flash-lite)
  -p, --persona PERSONA        AI Persona: neutral, butler, sergeant, hacker, pirate, genz (default: random)
  --completion SHELL           Output shell completion code for the specified shell (bash, zsh)
  -h, --help                   Display this help message
`
	fmt.Println(helpText)
}
