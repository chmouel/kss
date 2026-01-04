package main

import (
	"fmt"
	"os"
)

const bashCompletion = `_tkss_completions()
{
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    opts="-n --namespace -l --showlog --maxlines -w --watch --watch-interval -s --shell -f --follow --explain --model -p --persona -h --help --completion"

    case "${prev}" in
        -n|--namespace)
            local namespaces=$(kubectl get namespaces -o jsonpath='{.items[*].metadata.name}' 2>/dev/null)
            COMPREPLY=( $(compgen -W "${namespaces}" -- ${cur}) )
            return 0
            ;;
        --completion)
            local shells="bash zsh"
            COMPREPLY=( $(compgen -W "${shells}" -- ${cur}) )
            return 0
            ;;
        -p|--persona)
            local personas="neutral butler sergeant hacker pirate genz"
            COMPREPLY=( $(compgen -W "${personas}" -- ${cur}) )
            return 0
            ;;
        *)
            ;;
    esac

    if [[ ${cur} == -* ]] ; then
        COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
        return 0
    fi

    # PipelineRun completion
    local pipelineruns=$(kubectl get pipelineruns -o jsonpath='{.items[*].metadata.name}' 2>/dev/null)
    COMPREPLY=( $(compgen -W "${pipelineruns}" -- ${cur}) )
}
complete -F _tkss_completions tkss
`

const zshCompletion = `#compdef tkss

_tkss() {
    local -a opts
    opts=(
        {-n,--namespace}'[Use namespace]:namespace:_tkss_namespaces'
        {-l,--showlog}'[Show logs of TaskRun containers]'
        '--maxlines[Maximum line when showing logs]:int:'
        {-w,--watch}'[Watch mode (auto-refresh)]'
        '--watch-interval[Watch refresh interval in seconds]:seconds:'
        {-s,--shell}'[Open an interactive shell in a selected step]'
        {-f,--follow}'[Follow logs for a selected step]'
        '--explain[Enable AI explanation for PipelineRun failures]'
        '--model[AI model to use]:model:'
        {-p,--persona}'[AI Persona]:persona:(neutral butler sergeant hacker pirate genz)'
        {-h,--help}'[Display this help message]'
        '--completion[Output shell completion code]:shell:(bash zsh)'
    )
    _arguments -C $opts '*:pipelinerun:_tkss_pipelineruns'
}

_tkss_namespaces() {
    local -a namespaces
    namespaces=("${(@f)$(kubectl get namespaces -o jsonpath='{.items[*].metadata.name}' 2>/dev/null)}")
    compadd -a namespaces
}

_tkss_pipelineruns() {
    local -a pipelineruns
    pipelineruns=("${(@f)$(kubectl get pipelineruns -o jsonpath='{.items[*].metadata.name}' 2>/dev/null)}")
    compadd -a pipelineruns
}

_tkss
`

func printCompletion(shell string) {
	switch shell {
	case "bash":
		fmt.Print(bashCompletion)
	case "zsh":
		fmt.Print(zshCompletion)
	default:
		fmt.Printf("Error: unsupported shell '%s'. Available: bash, zsh\n", shell)
		os.Exit(1)
	}
}
