#!/bin/bash

COLOR_RED="\033[0;31m"
COLOR_CYAN="\033[0;36m"
COLOR_RESET="\033[0m"

RUN_CMD="go run main.go --config .develop/slurpeeth.yaml --debug"

go mod tidy
service docker start
sleep 1
docker pull debian:bookworm-slim
containerlab deploy -t .develop/clab.topo.yaml
sleep 5
.develop/setup-nodes.sh

echo -e "${COLOR_RED}
       __                              __   __
.-----|  .--.--.----.-----.-----.-----|  |_|  |--.
|__ --|  |  |  |   _|  _  |  -__|  -__|   _|     |
|_____|__|_____|__| |   __|_____|_____|____|__|__|
                    |__|

${COLOR_RESET}

To run: ${COLOR_CYAN}${RUN_CMD}${COLOR_RESET}

"

export HISTFILE=/tmp/.bash_history
history -s "$RUN_CMD"
history -a

bash