#!/usr/bin/env bash
set -e 

:<<'EOF'
    Well, this script is too short and easy.
    But I am lazy, so it born.
    If you use hexo too, you can try this script.
    chmod +x hexo_deploy.sh
    mv hexo_deploy.sh /usr/local/bin/hdeploy
EOF

# Input your codehub address.
WORKPWD=""
cd $WORKPWD

function p {
    git add .
    git commit -m "${NOTE:-'Update post'}"
    git push origin hexo
}

function d {
    hexo g && hexo d
    p
}

test $# -eq 0 && { d; exit 0; }


while test $# -ne 0
do
    cur="$1"
    next="$2"
    case "$cur" in
        -d)
            d
            echo "Deploy"
        ;;
        -m)
            case "$next" in
                -*)
                    NOTE="Update post."
                    ;;
                *)
                    NOTE="$next"
                    shift 
                    ;;
            esac
            echo "Commit Note: $NOTE"
        ;;
        -p)
            p
            echo "Push code."
        ;;
        *)
            printf "%s: No such option $cur.\n" ${0##*/} >&2
            echo "Please try other options." >&2
            exit 2
        ;;
    esac 
    shift
done

exit 0
