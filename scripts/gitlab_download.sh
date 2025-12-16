#!/bin/bash

:<<'EOF'
API from gitlab version 17.4
EOF

function old_get_params {
    proj_name=`echo "$url" | awk -F"/${1}/" '{print $1}' | xargs basename`
    branch=`echo "$url" | awk -F"${1}/" '{print $2}' | awk -F/ '{print $1}'`
    version_path=`echo "$url" | awk -F"/${1}/" '{print $2}' | awk -F "${branch}" '{print $2}'`
    gitlab_url=`echo "$url" | sed -E 's|^(https?://[^/]+).*|\1|'`
}

function new_get_params {
    proj_name=`echo "$url" | awk -F'/-/' '{print $1}' | xargs basename`
    branch=`echo "$url" | awk -F'/-/' '{print $2}' | awk -F/ '{print $2}'`
    version_path=`echo "$url" | awk -F"/-/" '{print $2}' | awk -F "${branch}" '{print $2}'`
    gitlab_url=`echo "$url" | sed -E 's|^(https?://[^/]+).*|\1|'`
}

function get_params {
    # echo "$url" | grep -q "?ref_type=heads" && { url="${url//\?ref_type*}"; new_get_params; } || old_get_params
    if echo "$url" | grep -q "/-/"; then
        url="${url/\?ref_type=heads}"
        new_get_params
    elif echo "$url" | grep -q "blob"; then
        old_get_params "blob"
    elif echo "$url" | grep -q "tree"; then
        old_get_params "tree"
    else
        echo "gitlab url is not correct"
        exit 1
    fi
}

function encode_to_url {
    orl_name=${1##*/}
    file_name=${orl_name%.*}
    file_postfix=${orl_name##*.}

    test "${file_postfix}" = "${file_name}" && file_postfix=""

    # Make the file name encode to url code.
    file_name=`echo -n "${file_name}" | od -An -tx1 | tr ' ' % | tr -d "\n"`

    test -z "${file_postfix}" && file_name_plus=${file_name} || file_name_plus=${file_name}.${file_postfix}

}

function get_files {
    proj_id=$(curl -s -H "PRIVATE-TOKEN: ${TOKEN}" "${gitlab_url}/api/v4/projects?search=${proj_name}" \
        | jq -r --arg proj_name "${proj_name}" '.[] | select(.name==$proj_name) | .id')

    path_list=$(curl -s -H "PRIVATE-TOKEN: ${TOKEN}" \
        "${gitlab_url}/api/v4/projects/${proj_id}/repository/tree?ref=${branch}&path=${version_path#/*}" | jq -r ".[].path")

    version_path=`echo ${version_path#/*} | sed 's#/#%2F#g'`
    IFS=$'\n'
    for item in ${path_list}
    do
        encode_to_url "${item}"
        download_files
    done
}

function download_files {
    file_url="${gitlab_url}/api/v4/projects/${proj_id}/repository/files/${version_path}%2F${file_name_plus}/raw?ref=${branch}"
    curl -sSL -f -H "PRIVATE-TOKEN: ${TOKEN}" "${file_url}" -o "${orl_name}"
    test $? -eq 0 || { echo "${orl_name} download failed"; exit 1; }
}

function options {
    while test $# -gt 0
    do
        case "$1" in
            -u|--url)
                url="$2"
                ;;
            -t|--token)
                TOKEN="$2"
                ;;
            *) Usage ;;
        esac
        shift 2
    done
}

Usage(){
    cat <<EOF
${0##*/} in order to download files from gitlab
    Usage:
        -u|--url       gitlab url.
        -t|--token     gitlab token.
EOF
}

main(){
    TOKEN=""
    options "$@"
    get_params
    get_files
}

main "$@"

exit 0