#!/bin/bash

test -d ./venv && source ./venv/Scripts/activate

function fetch_all(){
  local MODULE_NAMES="$@"

  for MODULE_NAME in $MODULE_NAMES
  do
    pip install $MODULE_NAME
    REQUIRES=$(pip show $MODULE_NAME | grep Requires | awk -F: '{print $2}' | tr ',' ' ')
    if test "$REQUIRES" = ""; then
      return
    else 
      fetch_all $REQUIRES
    fi
  done
}
echo "MODULE_NAME includes $MODULE_NAMES"
fetch_all "$@"

exit 0

:<<'EOF'
Only for windows git-bash.
Recursion download.
Move this scripts to your relative path of project venv directory.
EOF