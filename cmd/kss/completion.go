package main

import (
	"fmt"
	"os"
)

const bashCompletion = `_kss_completions()
{
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    opts="-n --namespace -r --restrict -l --showlog --maxlines -L --labels -A --annotations -E --events -w --watch --watch-interval -d --doctor -s --shell --explain --model -p --persona -h --help --completion"

    case "${prev}" in
        -n|--namespace)
            local namespaces=$(kubectl get namespaces -o jsonpath='{.items[*].metadata.name}' 2>/dev/null)
            COMPREPLY=( $(compgen -W "${namespaces}" -- ${cur}) )
            return 0
            ;;
        -p|--persona)
            local personas="neutral butler sergeant hacker pirate genz"
            COMPREPLY=( $(compgen -W "${personas}" -- ${cur}) )
            return 0
            ;;
        --completion)
            local shells="bash zsh"
            COMPREPLY=( $(compgen -W "${shells}" -- ${cur}) )
            return 0
            ;;
        *)
            ;;
    esac

    if [[ ${cur} == -* ]] ; then
        COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
        return 0
    fi

    # Pod completion
    local pods=$(kubectl get pods -o jsonpath='{.items[*].metadata.name}' 2>/dev/null)
    COMPREPLY=( $(compgen -W "${pods}" -- ${cur}) )
}
complete -F _kss_completions kss
`

const zshCompletion = `#compdef kss

_kss() {
    local -a opts
    opts=(
        {-n,--namespace}'[Use namespace]:namespace:_kss_namespaces'
        {-r,--restrict}'[Restrict to show only those containers (regexp)]:regexp:'
        {-l,--showlog}'[Show logs of containers]'
        '--maxlines[Maximum line when showing logs]:int:'
        {-L,--labels}'[Show labels]'
        {-A,--annotations}'[Show annotations]'
        {-E,--events}'[Show events]'
        {-w,--watch}'[Watch mode (auto-refresh)]'
        '--watch-interval[Watch refresh interval in seconds]:seconds:'
        {-d,--doctor}'[Enable heuristic analysis (Doctor mode)]'
        {-s,--shell}'[Open an interactive shell in the selected pod]'
        '--explain[Enable AI explanation for pod failures]'
        '--model[AI Model to use]:model:'
        {-p,--persona}'[AI Persona]:persona:(neutral butler sergeant hacker pirate genz)'
        {-h,--help}'[Display this help message]'
        '--completion[Output shell completion code]:shell:(bash zsh)'
    )
    _arguments -C $opts '*:pod:_kss_pods'
}

_kss_namespaces() {
    local -a namespaces
    namespaces=("${(@f)$(kubectl get namespaces -o jsonpath='{.items[*].metadata.name}' 2>/dev/null)}")
    compadd -a namespaces
}

_kss_pods() {
    local -a pods
    pods=("${(@f)$(kubectl get pods -o jsonpath='{.items[*].metadata.name}' 2>/dev/null)}")
    compadd -a pods
}

_kss
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
