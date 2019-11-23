# KSS - Kubernetes pod status on steroid

A simple tool to show the current status of the pod and its associated `containers` and `initContainers`.

You can specify the pods to get status for as argument if you don't it will launch [fzf](https://github.com/junegunn/fzf) and let choose it (or select automatically the first available if there is only one).

If you specify the `-l` option it will shows the output log as well, you can adjust how many line of th log you like with the `--maxlines=INT`.

You can use the `-r` option with a regexp to restrict the status (or the log output) to certain containers.


## Screenshots
