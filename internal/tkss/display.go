package tkss

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"

	"github.com/chmouel/kss/internal/ai"
	"github.com/chmouel/kss/internal/kube"
	"github.com/chmouel/kss/internal/model"
	"github.com/chmouel/kss/internal/tekton"
	"github.com/chmouel/kss/internal/util"
)

// PrintPipelineRun renders a PipelineRun summary and optional TaskRun logs.
func PrintPipelineRun(pr tekton.PipelineRun, taskRuns []tekton.TaskRun, args Args, kctl string, kubectlArgs []string) {
	label, color, reason, message := tekton.StatusLabel(pr.Status.Conditions)
	pipelineName := "inline"
	if pr.Spec.PipelineRef != nil && pr.Spec.PipelineRef.Name != "" {
		pipelineName = pr.Spec.PipelineRef.Name
	}

	statusText := util.ColorText(label, color)
	if reason != "" && label != "Succeeded" {
		statusText = fmt.Sprintf("%s (%s)", statusText, reason)
	}

	width := util.TerminalWidth()
	if args.Preview && width > 100 {
		width = 100
	}
	if width < 60 {
		width = 60
	}

	summary := table.NewWriter()
	if args.Preview {
		summary.SetStyle(table.StyleLight)
	} else {
		summary.SetStyle(table.StyleRounded)
	}
	summary.SetAllowedRowLength(width)
	summary.SetTitle(util.ColorText("PipelineRun", "cyan"))
	summary.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight},
		{Number: 2, Align: text.AlignLeft, WidthMax: width - 24},
	})
	summary.AppendRow(table.Row{util.ColorText("Name", "dim"), util.ColorText(pr.Metadata.Name, "white_bold")})
	summary.AppendRow(table.Row{util.ColorText("Namespace", "dim"), pr.Metadata.Namespace})
	summary.AppendRow(table.Row{util.ColorText("Pipeline", "dim"), pipelineName})
	summary.AppendRow(table.Row{util.ColorText("Status", "dim"), statusText})
	if message != "" {
		summary.AppendRow(table.Row{util.ColorText("Message", "dim"), message})
	}
	summary.AppendRow(table.Row{util.ColorText("Age", "dim"), util.FormatDuration(pr.Metadata.CreationTimestamp)})
	summary.AppendRow(table.Row{util.ColorText("Duration", "dim"), formatDurationBetween(pr.Status.StartTime, pr.Status.CompletionTime)})
	fmt.Println(summary.Render())

	if len(taskRuns) > 0 {
		fmt.Println()

		rows := make([]tekton.TaskRun, 0, len(taskRuns))
		rows = append(rows, taskRuns...)
		slices.SortFunc(rows, func(a, b tekton.TaskRun) int {
			return strings.Compare(tekton.TaskRunDisplayName(a), tekton.TaskRunDisplayName(b))
		})

		tw := table.NewWriter()
		if args.Preview {
			tw.SetStyle(table.StyleLight)
		} else {
			tw.SetStyle(table.StyleRounded)
		}
		tw.SetAllowedRowLength(width)
		tw.Style().Options.SeparateRows = !args.Preview
		tw.Style().Color.Header = text.Colors{text.FgCyan, text.Bold}
		tw.SetTitle(util.ColorText("Tasks", "cyan"))
		tw.SetColumnConfigs([]table.ColumnConfig{
			{Number: 1, Align: text.AlignLeft},
			{Number: 2, Align: text.AlignLeft},
			{Number: 3, Align: text.AlignLeft},
			{Number: 4, Align: text.AlignRight},
			{Number: 5, Align: text.AlignRight},
			{Number: 6, Align: text.AlignLeft},
		})
		tw.AppendHeader(table.Row{"Task", "TaskRun", "Status", "Age", "Duration", "Pod"})
		for i := range rows {
			tr := &rows[i]
			statusLabel, statusColor, statusReason, _ := tekton.StatusLabel(tr.Status.Conditions)
			statusText := util.ColorText(statusLabel, statusColor)
			if statusReason != "" && statusLabel != "Succeeded" {
				statusText = fmt.Sprintf("%s (%s)", statusText, statusReason)
			}

			tw.AppendRow(table.Row{
				tekton.TaskRunDisplayName(*tr),
				tr.Metadata.Name,
				statusText,
				util.FormatDuration(tr.Status.StartTime),
				formatDurationBetween(tr.Status.StartTime, tr.Status.CompletionTime),
				tr.Status.PodName,
			})
		}
		fmt.Println(tw.Render())
	} else {
		fmt.Println()
		fmt.Println(util.ColorText("No TaskRuns found for this PipelineRun.", "yellow"))
	}

	if args.ShowLog && !args.Preview && len(taskRuns) > 0 {
		fmt.Println()
		PrintTaskRunLogs(taskRuns, args, kctl, kubectlArgs)
	}

	if args.Explain && !args.Preview {
		ai.ExplainPipelineRun(pr, taskRuns, kctl, kubectlArgs, model.Args{
			MaxLines: args.MaxLines,
			Model:    args.Model,
			Persona:  args.Persona,
		})
	}
}

// PrintTaskRunLogs prints container logs for each TaskRun in the list.
func PrintTaskRunLogs(taskRuns []tekton.TaskRun, args Args, kctl string, kubectlArgs []string) {
	for i := range taskRuns {
		tr := &taskRuns[i]
		podName, err := tekton.PodNameForTaskRun(kubectlArgs, *tr)
		if err != nil {
			fmt.Printf("%s %s: %v\n", util.ColorText("Logs:", "cyan"), tr.Metadata.Name, err)
			continue
		}

		podObj, err := kube.FetchPod(kubectlArgs, podName)
		if err != nil {
			fmt.Printf("%s %s: %v\n", util.ColorText("Logs:", "cyan"), tr.Metadata.Name, err)
			continue
		}

		fmt.Printf("%s %s (%s)\n", util.ColorText("Logs:", "cyan"), tr.Metadata.Name, podName)

		for _, container := range podObj.Spec.Containers {
			output, err := kube.ShowLog(kctl, model.Args{MaxLines: args.MaxLines}, container.Name, podName, false)
			if err != nil || output == "" {
				continue
			}
			fmt.Printf("  %s\n", util.ColorText(fmt.Sprintf("Container %s:", container.Name), "cyan"))
			fmt.Println(util.ColorText("  --------------------------------------------------------------", "dim"))
			for _, line := range strings.Split(output, "\n") {
				fmt.Printf("  %s\n", line)
			}
			fmt.Println(util.ColorText("  --------------------------------------------------------------", "dim"))
		}
		fmt.Println()
	}
}

func formatDurationBetween(start, end string) string {
	if start == "" {
		return "N/A"
	}
	startTime, err := time.Parse(time.RFC3339, start)
	if err != nil {
		return "N/A"
	}
	endTime := time.Now()
	if end != "" {
		if parsed, err := time.Parse(time.RFC3339, end); err == nil {
			endTime = parsed
		}
	}

	duration := endTime.Sub(startTime)
	switch {
	case duration < time.Minute:
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	case duration < time.Hour:
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	case duration < 24*time.Hour:
		return fmt.Sprintf("%dh", int(duration.Hours()))
	default:
		return fmt.Sprintf("%dd", int(duration.Hours()/24))
	}
}
