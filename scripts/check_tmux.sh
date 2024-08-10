#!/usr/bin/env bash
:'EOF'
    This script is using to auto boot tmux when you login shell. 
    I am lazy, so it boom.
    If you use tmux and lazy too, you can try.

    mv check_tmux.sh /usr/local/bin/check_tmux
    echo "/usr/local/bin/check_tmux" >> ~/.bashrc

EOF

set -e

test -n "$TMUX" && { echo "Already in tmux."; exit; }

test $(type -P tmux) && { session=$(tmux ls | awk -F: 'NR==1 {print $1}'); } 2> /dev/null

test -n "$session" && tmux attach -t $session || tmux
