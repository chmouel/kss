package kube

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/chmouel/kss/internal/model"
	"github.com/chmouel/kss/internal/util"
)

// KubectlArgs constructs basic kubectl arguments including namespace
func KubectlArgs(args model.Args) []string {
	if args.Namespace == "" {
		return nil
	}
	return []string{"-", "n", args.Namespace}
}

// FetchPod retrieves pod details from Kubernetes
func FetchPod(kubectlArgs []string, pod string) (model.Pod, error) {
	cmdArgs := append(append([]string{}, kubectlArgs...), "get", "pod", pod, "-ojson")
	cmd := exec.Command("kubectl", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return model.Pod{}, fmt.Errorf("could not fetch pod %s: %s", pod, strings.TrimSpace(string(output)))
	}

	var podObj model.Pod
	if err := json.Unmarshal(output, &podObj); err != nil {
		return model.Pod{}, fmt.Errorf("could not parse pod data for %s: %w", pod, err)
	}

	return podObj, nil
}

// ListPods retrieves all pods in the current namespace
func ListPods(kubectlArgs []string) ([]model.Pod, error) {
	cmdArgs := append(append([]string{}, kubectlArgs...), "get", "pods", "-ojson")
	cmd := exec.Command("kubectl", cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("could not list pods: %s", strings.TrimSpace(string(output)))
	}

	var podList model.PodList
	if err := json.Unmarshal(output, &podList); err != nil {
		return nil, fmt.Errorf("could not parse pod list: %w", err)
	}

	return podList.Items, nil
}

// ShowLog retrieves and returns container logs using kubectl
func ShowLog(kctl string, args model.Args, container, pod string, previous bool) (string, error) {
	cmdArgs := fmt.Sprintf("%s logs --tail=%s %s -c%s", kctl, args.MaxLines, pod, container)
	if previous {
		cmdArgs += " -p"
	}
	cmd := exec.Command("sh", "-c", cmdArgs)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("could not run '%s': %v", cmd.String(), err)
	}
	return strings.TrimSpace(string(output)), nil
}

type containerInfo struct {
	Name    string
	State   string
	Running bool
}

func containerInfoForPod(kubectlArgs []string, pod string) ([]containerInfo, error) {
	podObj, err := FetchPod(kubectlArgs, pod)
	if err != nil {
		return nil, err
	}

	statusByName := make(map[string]containerInfo, len(podObj.Status.ContainerStatuses))
	for _, status := range podObj.Status.ContainerStatuses {
		state, running := status.StateLabel()
		statusByName[status.Name] = containerInfo{
			Name:    status.Name,
			State:   state,
			Running: running,
		}
	}

	infos := make([]containerInfo, 0, len(podObj.Spec.Containers))
	for _, container := range podObj.Spec.Containers {
		if info, ok := statusByName[container.Name]; ok {
			infos = append(infos, info)
			continue
		}
		infos = append(infos, containerInfo{Name: container.Name, State: "unknown"})
	}

	return infos, nil
}

func selectContainerWithFzf(containers []string) (string, error) {
	cmd := exec.Command("fzf", "-0", "-1", "--prompt", "container> ")
	cmd.Stdin = strings.NewReader(strings.Join(containers, "\n"))
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("fzf selection failed: %w", err)
	}

	selection := strings.TrimSpace(output.String())
	if selection == "" {
		return "", fmt.Errorf("no container selected")
	}

	return selection, nil
}

func selectContainerByPrompt(containers []string) (string, error) {
	fmt.Println("Select a container:")
	for i, name := range containers {
		fmt.Printf("  %d) %s\n", i+1, name)
	}
	fmt.Print("Enter number: ")

	reader := bufio.NewReader(os.Stdin)
	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read selection: %w", err)
		}
		input = strings.TrimSpace(input)
		if input == "" {
			fmt.Print("Enter number: ")
			continue
		}

		index, err := strconv.Atoi(input)
		if err != nil || index < 1 || index > len(containers) {
			fmt.Printf("Please enter a number between 1 and %d: ", len(containers))
			continue
		}

		return containers[index-1], nil
	}
}

func selectContainer(containers []string, restrict string) (string, error) {
	filtered, err := util.FilterContainersByRestrict(containers, restrict)
	if err != nil {
		return "", err
	}

	if len(filtered) == 1 {
		return filtered[0], nil
	}

	if util.Which("fzf") != "" {
		return selectContainerWithFzf(filtered)
	}

	return selectContainerByPrompt(filtered)
}

var shellCandidates = []string{
	"/bin/bash",
	"/usr/bin/bash",
	"/bin/sh",
	"/usr/bin/sh",
	"/bin/ash",
	"/busybox/sh",
	"/bin/dash",
}

func probeShell(kubectlArgs []string, pod, container, shell string) bool {
	cmdArgs := append(append([]string{}, kubectlArgs...), "exec", pod, "-c", container, "--", shell, "-c", "exit 0")
	cmd := exec.Command("kubectl", cmdArgs...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

func pickShell(kubectlArgs []string, pod, container string) (string, error) {
	for _, shell := range shellCandidates {
		if probeShell(kubectlArgs, pod, container, shell) {
			return shell, nil
		}
	}
	return "", fmt.Errorf("no usable shell found in pod %s (tried: %s)", pod, strings.Join(shellCandidates, ", "))
}

// RunShell interacts with the user to open a shell in a pod's container
func RunShell(args model.Args, kubectlArgs []string, pod string) error {
	containers, err := containerInfoForPod(kubectlArgs, pod)
	if err != nil {
		return err
	}
	if len(containers) == 0 {
		return fmt.Errorf("pod %s has no containers to shell into", pod)
	}

	allNames := make([]string, 0, len(containers))
	runningNames := make([]string, 0, len(containers))
	stateByName := make(map[string]string, len(containers))
	runningByName := make(map[string]bool, len(containers))
	for _, container := range containers {
		allNames = append(allNames, container.Name)
		stateByName[container.Name] = container.State
		runningByName[container.Name] = container.Running
		if container.Running {
			runningNames = append(runningNames, container.Name)
		}
	}

	candidates := allNames
	if args.Restrict == "" {
		if len(runningNames) == 0 {
			var details []string
			for _, name := range allNames {
				details = append(details, fmt.Sprintf("%s (%s)", name, stateByName[name]))
			}
			return fmt.Errorf("no running containers in pod %s (%s)", pod, strings.Join(details, ", "))
		}
		candidates = runningNames
	}

	container, err := selectContainer(candidates, args.Restrict)
	if err != nil {
		return err
	}
	if !runningByName[container] {
		state := stateByName[container]
		if state == "" {
			state = "unknown"
		}
		return fmt.Errorf("container %s is not running (%s)", container, state)
	}

	shell, err := pickShell(kubectlArgs, pod, container)
	if err != nil {
		return err
	}
	if !util.IsBashShell(shell) {
		fmt.Printf("Using %s (bash unavailable)\n", shell)
	}

	cmdArgs := append(append([]string{}, kubectlArgs...), "exec", "-it", pod, "-c", container, "--", shell)
	cmd := exec.Command("kubectl", cmdArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}
