# KSS - Kubernetes pod status on steroid

A simple tool to show the current status of the pod and its associated `containers` and `initContainers`. This was developed out of frustration with `kubectl get pod` not showing much and `kubectl describe pod` showing way too much in a cryptic way. Debugging failed pods with a lot of `initContainers` and `sideCars` usually was done with `kubectl get pod -o yaml |less` wiht a lot of going up and down and a bunch of censored swearing 🔞 to figure out what's going on. All those techniques for introspection and debugging are still useful  and KSS is not planning to replace them but now I swear less 😅

## Usage

You can specify a pod or multiple ones to get the status as argument to KSS, if you don't it will launch [fzf](https://github.com/junegunn/fzf) and let you choose it interactively (or select automatically the first available if there is only one), use the [TAB] to select multiple pods. KSS would use itself if it find itself in the `PATH` for the FZF preview window or it will fallback to a boring ol' `kubectl describe`.

If you specify the `-l` option it will show the output log as well, you can adjust how many line of the log you want to see with the flag `--maxlines=INT`.

You can use the `-r` option with a regexp to restrict the status (or the log output) to certain containers.

## Install

You just make sure you have >python3.6, fzf and kubctl. You then can download the [script](https://raw.githubusercontent.com/chmouel/kss/master/kss) and put directly into your filesystem path or checkout this GIT repo and link the binary into your path so you can have the updates. 

With zsh you can install the [_kss](./_kss) completionfile  to your [fpath](https://unix.stackexchange.com/a/33898).

I may do a [krew](https://github.com/kubernetes-sigs/krew) plugin and/or brew homebrew repository if this get popular enough.

## Screenshots

### Success run

![Success run](.screenshots/success.png)

### Failed run

![Fail run](.screenshots/failure.png)

### Failed run with logs

![Fail run](.screenshots/logging.png)

### Restrict to show logs only to certain container and only one line

![Restrict to some pod](.screenshots/restrict.png)

### Selecting a pod with fzf

[![Select a pod with FZF](https://asciinema.org/a/WNBiFbv0ExwPFsqPP9lvEx0SY.png)](https://asciinema.org/a/WNBiFbv0ExwPFsqPP9lvEx0SY)


## Misc

* The code is currently quite humm simple and stupid, the kind of stuff you start to write quickly and dirty and it grows it grows until it really become a unreadable beast. I probably going to rewrite it up properly with tests and all (in a compiled language? soonishly enough. Byt hey who cares since it works 😅
