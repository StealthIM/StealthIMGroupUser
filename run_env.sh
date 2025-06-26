#!/bin/bash

tmux new-session -d -s stimenv 'cd ../StealthIMDB && make'
tmux select-window -t stimenv:0
tmux split-window -v 'cd ../StealthIMSession && make'
tmux split-window -v 'cd ../StealthIMUser && make'
tmux attach-session -t stimenv