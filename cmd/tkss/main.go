// Package main implements TKSS - Enhanced Tekton PipelineRun inspection.
package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/kss/internal/kube"
	"github.com/chmouel/kss/internal/model"
	"github.com/chmouel/kss/internal/tekton"
	"github.com/chmouel/kss/internal/tkss"
	"github.com/chmouel/kss/internal/tui"
	"github.com/chmouel/kss/internal/util"
)

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func main() {
	args := parseArgs()

	if args.Completion != "" {
		printCompletion(args.Completion)
		os.Exit(0)
	}

	for i, pipelineRun := range args.PipelineRuns {
		args.PipelineRuns[i] = util.ResourceName(pipelineRun)
	}

	if err := tkss.RequireFzf(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	kctl := "kubectl"
	if args.Namespace != "" {
		kctl = fmt.Sprintf("kubectl -n %s", args.Namespace)
	}
	kubectlArgs := kubectlArgs(args.Namespace)

	if args.Preview {
		if len(args.PipelineRuns) == 0 {
			fmt.Println("preview requires a pipelinerun name")
			os.Exit(1)
		}
		previewPipelineRun(args, args.PipelineRuns[0], kctl, kubectlArgs)
		return
	}

	if len(args.PipelineRuns) == 0 && !args.Shell && !args.Follow && !args.Watch {
		m := tui.NewModel("pipelinerun", args.Namespace, kubectlArgs)
		p := tea.NewProgram(m)
		finalModel, err := p.Run()
		if err != nil {
			fmt.Printf("Error running TUI: %v\n", err)
			os.Exit(1)
		}

		tm := finalModel.(tui.Model)
		if tm.ChosenItem != nil {
			args.PipelineRuns = []string{tm.ChosenItem.FilterValue()}
		} else {
			return
		}
	}

	pipelineRuns, err := resolvePipelineRuns(args, kctl)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if len(pipelineRuns) == 0 {
		fmt.Println("No PipelineRuns found.")
		os.Exit(1)
	}

	if args.Shell {
		pipelineRun := pipelineRuns[0]
		if err := runShell(args, pipelineRun, kubectlArgs); err != nil {
			fmt.Printf("Unable to start shell for pipelinerun %s: %v\n", pipelineRun, err)
			os.Exit(1)
		}
		return
	}

	if args.Follow {
		pipelineRun := pipelineRuns[0]
		if err := followLogs(args, pipelineRun, kctl, kubectlArgs); err != nil {
			fmt.Printf("Unable to follow logs for pipelinerun %s: %v\n", pipelineRun, err)
			os.Exit(1)
		}
		return
	}

	if args.Watch {
		watchPipelineRuns(args, pipelineRuns, kctl, kubectlArgs)
		return
	}

	for _, pipelineRun := range pipelineRuns {
		pr, taskRuns, err := fetchPipelineRun(args, pipelineRun, kubectlArgs)
		if err != nil {
			fmt.Printf("Error fetching pipelinerun %s: %v\n", pipelineRun, err)
			continue
		}

		tkss.PrintPipelineRun(pr, taskRuns, args, kctl, kubectlArgs)
		if len(pipelineRuns) > 1 {
			fmt.Println()
		}
	}
}

func parseArgs() tkss.Args {
	args := tkss.Args{
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
		case "-l", "--showlog":
			args.ShowLog = true
		case "--maxlines":
			if i+1 < len(os.Args) {
				args.MaxLines = os.Args[i+1]
				i++
			}
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
		case "-s", "--shell":
			args.Shell = true
		case "-f", "--follow":
			args.Follow = true
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
				args.PipelineRuns = append(args.PipelineRuns, arg)
			}
		}
	}

	if args.Persona == "" {
		// #nosec G404
		args.Persona = personas[rand.Intn(len(personas))]
	}

	return args
}

func printHelp() {
	helpText := `
TKSS - Tekton PipelineRun Status
Enhanced Tekton PipelineRun Inspection

Usage: tkss [OPTIONS] [PIPELINERUN...]

Options:
  -n, --namespace NAMESPACE    Use namespace
  -l, --showlog                Show logs of TaskRun containers
  --maxlines INT               Maximum line when showing logs (default: -1)
  -w, --watch                  Watch mode (auto-refresh)
  --watch-interval SECONDS     Watch refresh interval in seconds (default: 2)
  -s, --shell                  Open an interactive shell in a selected step
  -f, --follow                 Follow logs for a selected step
  --explain                    Enable AI explanation for PipelineRun failures
  --model MODEL                AI model to use (default: gemini-2.5-flash-lite)
  -p, --persona PERSONA        AI persona: neutral, butler, sergeant, hacker, pirate, genz (default: random)
  --completion SHELL           Output shell completion code for the specified shell (bash, zsh)
  -h, --help                   Display this help message
`
	fmt.Println(helpText)
}

func kubectlArgs(namespace string) []string {
	if namespace == "" {
		return nil
	}
	return []string{"-n", namespace}
}

func resolvePipelineRuns(args tkss.Args, kctl string) ([]string, error) {
	if len(args.PipelineRuns) > 0 {
		return args.PipelineRuns, nil
	}

	preview := fmt.Sprintf("%s describe pipelinerun {}", kctl)
	myself, err := os.Executable()
	if err != nil || myself == "" {
		myself = util.Which("tkss")
	}
	if myself != "" {
		preview = myself + " --preview"
		if args.Namespace != "" {
			preview += fmt.Sprintf(" -n %s", args.Namespace)
		}
		preview += " {}"
	}

	runcmd := fmt.Sprintf("%s get pipelineruns -o name|fzf -0 -n 1 -m -1 --preview-window 'down:60%%:wrap' --preview='%s'", kctl, preview)
	cmd := exec.Command("sh", "-c", runcmd)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("no pipelineruns is no news which is arguably no worries")
	}

	items := strings.TrimSpace(string(output))
	if items == "" {
		return nil, fmt.Errorf("no pipelineruns is no news which is arguably no worries")
	}

	lines := strings.Split(items, "\n")
	pipelineRuns := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pipelineRuns = append(pipelineRuns, util.ResourceName(line))
	}
	return pipelineRuns, nil
}

func fetchPipelineRun(args tkss.Args, name string, kubectlArgs []string) (tekton.PipelineRun, []tekton.TaskRun, error) {
	pr, err := tekton.FetchPipelineRun(kubectlArgs, name)
	if err != nil {
		return tekton.PipelineRun{}, nil, err
	}
	if pr.Metadata.Namespace == "" {
		pr.Metadata.Namespace = args.Namespace
	}

	taskRuns, err := tekton.FetchTaskRunsForPipelineRun(kubectlArgs, name)
	if err != nil {
		return tekton.PipelineRun{}, nil, err
	}

	return pr, taskRuns, nil
}

func previewPipelineRun(args tkss.Args, name, kctl string, kubectlArgs []string) {
	pr, taskRuns, err := fetchPipelineRun(args, name, kubectlArgs)
	if err != nil {
		fmt.Printf("Error fetching pipelinerun %s: %v\n", name, err)
		return
	}

	args.Preview = true
	tkss.PrintPipelineRun(pr, taskRuns, args, kctl, kubectlArgs)
}

func watchPipelineRuns(args tkss.Args, pipelineRuns []string, kctl string, kubectlArgs []string) {
	interval := time.Duration(args.WatchInterval) * time.Second
	if interval == 0 {
		interval = 2 * time.Second
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-sigChan:
			fmt.Println("\nExiting watch mode...")
			os.Exit(0)
		default:
			clearScreen()
			fmt.Printf("%s Watching pipelineruns (refresh every %v, press Ctrl+C to exit)\n\n",
				util.ColorText("watch", "cyan"), interval)

			for _, pipelineRun := range pipelineRuns {
				pr, taskRuns, err := fetchPipelineRun(args, pipelineRun, kubectlArgs)
				if err != nil {
					fmt.Printf("Error fetching pipelinerun %s: %v\n", pipelineRun, err)
					continue
				}

				tkss.PrintPipelineRun(pr, taskRuns, args, kctl, kubectlArgs)
				fmt.Println()
			}

			time.Sleep(interval)
		}
	}
}

func runShell(args tkss.Args, pipelineRun string, kubectlArgs []string) error {
	_, taskRuns, err := fetchPipelineRun(args, pipelineRun, kubectlArgs)
	if err != nil {
		return err
	}

	targets, err := tkss.BuildStepTargets(kubectlArgs, taskRuns)
	if err != nil {
		return err
	}

	target, err := tkss.SelectStepTarget(targets)
	if err != nil {
		return err
	}

	restrict := fmt.Sprintf("^%s$", regexp.QuoteMeta(target.ContainerName))
	return kube.RunShell(model.Args{Restrict: restrict}, kubectlArgs, target.PodName)
}

func followLogs(args tkss.Args, pipelineRun, kctl string, kubectlArgs []string) error {
	_, taskRuns, err := fetchPipelineRun(args, pipelineRun, kubectlArgs)
	if err != nil {
		return err
	}

	targets, err := tkss.BuildStepTargets(kubectlArgs, taskRuns)
	if err != nil {
		return err
	}

	target, err := tkss.SelectStepTarget(targets)
	if err != nil {
		return err
	}

	cmdArgs := fmt.Sprintf("%s logs -f --tail=%s -c %s %s", kctl, args.MaxLines, target.ContainerName, target.PodName)
	cmd := exec.Command("sh", "-c", cmdArgs)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
