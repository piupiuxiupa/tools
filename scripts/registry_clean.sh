#!/bin/bash/env bash

## Don't add / in the url end, like https://127.0.0.1/
REGISTRY=""
VERSION="v2"
CATALOG="_catalog"
TAGS_LIST="tags/list"

REGISTRY_CONTAINER_NAME="${REGISTRY_CONTAINER_NAME:-registry}"
REGISTRY_CONF="${REGISTRY_CONF:-"/etc/docker/registry/config.yml"}"
REGISTRY_DIR=""
SUB_DIR="docker/registry/v2"

JSON_BAK_FILE="registry.json"
USERNAME=""
PASSWD=""

BACKUP_NUM=1

FLAG_A=1
FLAG_B=1
FLAG_C=1
FLAG_D=1
FLAG_S=1

# Use -A to define var similar python dict
declare -A IMAGE_TAGS
declare -A PID_LIST

function check_images {

    IMAGES=($(curl -sS -L -k -u $USERNAME:$PASSWD $REGISTRY/$VERSION/$CATALOG | jq -r '.repositories | join(" ")'))

    test -f $JSON_BAK_FILE && mv $JSON_BAK_FILE ${JSON_BAK_FILE}_$(date "+%s")
    for image in ${IMAGES[@]}; do
        
        # echo "$USERNAME:$PASSWD $REGISTRY/$VERSION/$image/$TAGS_LIST"
        curl -sS -L -k -u $USERNAME:$PASSWD --connect-timeout 5 --max-time 10 \
            $REGISTRY/$VERSION/$image/$TAGS_LIST | jq | tee -a ${JSON_BAK_FILE} &
        image=${image//\//_}

        IMAGE_TAGS["$image"]=$(curl -sSL -k -u $USERNAME:$PASSWD $REGISTRY/$VERSION/${image//_/\/}/$TAGS_LIST | jq -r '.tags | join(" ")')
        echo "$image version: ${IMAGE_TAGS["$image"]}"
        wait
    done
}


function wait_jobs {
    # wait -n, when a job is done, it will output status of the job.
    for pid in $(jobs -rp); do
        wait -n
        test $? -eq 0 && echo "$pid -- done."
    done
}

function backup_images {


    test -n "$USERNAME" -a -n "$PASSWD" && docker login $REGISTRY -u $USERNAME -p $PASSWD

    
    for image in ${IMAGES[@]}; do
        image=${image//\//_}

        
        TAGS=(${IMAGE_TAGS["$image"]})
        # echo "image tags ---------- ${TAGS[@]:0:$BACKUP_NUM}"
        
        # ${TAGS[@]:0:3} fetch three image tags of head
        for tag in ${TAGS[@]:0:$BACKUP_NUM}; do
            docker pull ${REGISTRY##*/}/${image//_/\/}:$tag >/dev/null &
            # PID_LIST["${image},${tag}"]=$!
        done
    done

    # for image in ${IMAGES[@]}
    # do
    #     image=${image//\//_}
    #     for tag in ${IMAGE_TAGS["$image"]}
    #     do
    #         # echo "PIDLIST is -----  ${PID_LIST["$image","$tag"]}"
    #         PIDS+=(${PID_LIST["$image","$tag"]})
    #     done
    # done
    echo "Image pull start"
    wait_jobs
    echo "Image pull successfully !"
}

function delete_registry {
    HEADER="Accept: application/vnd.docker.distribution.manifest.v2+json"

    for image in ${IMAGES[@]}; do
        image=${image//\//_}

        for tag in ${IMAGE_TAGS["$image"]}; do
            DIGEST=$(curl --connect-timeout 5 --max-time 10 -u $USERNAME:$PASSWD \
                -sSL -H "$HEADER" $REGISTRY/$VERSION/${image//_/\/}/manifests/$tag |
                jq -r ".config.digest")
            curl -sSLf -k -X DELETE -u $USERNAME:$PASSWD $REGISTRY/$VERSION/${image//_/\/}/manifests/$DIGEST

            # Fake output: If DIGEST not exsits, status output 0 too. So use curl -f to stdout and recall non-zero status
            test $? -eq 0 && echo "Delete $image $tag digest successfully."
        done
    done

    
    docker exec "$REGISTRY_CONTAINER_NAME" registry garbage-collect -m "$REGISTRY_CONF" || exit 1

    cd ${REGISTRY_DIR}/${SUB_DIR} || exit
    pwd
    if test "${REGISTRY_DIR}/${SUB_DIR}" = "$PWD"; then
        rm -rf repositories/* blobs/*
        docker restart $REGISTRY_CONTAINER_NAME
        test $? -eq 0 && echo "Registry restart successfully."
    else
        echo "No such directory ${REGISTRY_DIR}/${SUB_DIR}"
        exit
    fi
}

function push_image {
    # echo "push ----- ${IMAGES[@]}"
    for image in ${IMAGES[@]}; do
        image=${image//\//_}
        # echo "push image ---------  $image"
        TAGS=(${IMAGE_TAGS["$image"]})
        for tag in ${TAGS[@]:0:$BACKUP_NUM}; do
            docker push ${REGISTRY##*/}/${image//_/\/}:$tag &
        done
    done
    # echo `jobs -rp`
    # Fake output: wait for jobs list, only one job status of this list is 0, $? is 0
    # wait `jobs -rp`
    echo "Image push start"
    wait_jobs
    echo "Image push successfully !"
}

function clean_local_images {
    docker images -q | xargs -i -P0 docker rmi {}
}

function sub_options {
    for ((i=1; i<${#1}; i++)); do
        case ${1:${i}:1} in
            a) FLAG_A=0 ;;
            b) FLAG_B=0 ;;
            c) FLAG_C=0 ;;
            d) FLAG_D=0 ;;
            s) FLAG_S=0 ;;
            n) test $((i+1)) -eq ${#1} && BACKUP_NUM=${2:-1} ;;
            f) test $((i+1)) -eq ${#1} && JSON_BAK_FILE="${2:-'registry.json'}" ;;
            p) test $((i+1)) -eq ${#1} && PASSWD="${2:?'Password is empty.'}" || { echo "Please input password." >&2 ; exit; };;
            r) test $((i+1)) -eq ${#1} && REGISTRY="${2:?'Registry url is empty.'}" || { echo "Please input url." >&2 ; exit; } ;;
            u) test $((i+1)) -eq ${#1} && USERNAME="${2:?'Username is empty.'}" || { echo "Please input username." >&2 ; exit; } ;;
            x) test $((i+1)) -eq ${#1} && REGISTRY_DIR="${2:?'Registry path is empty.'}" || { echo "Please input registry path." >&2; exit; } ;;
            h) Usage; exit 1 ;;
            *) echo -e "No such options -${1:${i}:1}. \nPlease read help manual." >&2 ; exit 1 ;;
        esac
    done
}

function options {
    while test $# -gt 0; do
        case "$1" in
        -a) FLAG_A=0 ;;
        -b) FLAG_B=0 ;;
        -c) backup_images;;
        -d) FLAG_D=0 ;;
        -s) FLAG_S=0 ;;
        -n) 
            if [[ "$2" != -* && $2 =~ ^[0-9]+$ ]]; then
                BACKUP_NUM=$2 
                shift
            fi
            ;;
        -f) 
            if [[ "$2" != -* ]]; then
                JSON_BAK_FILE="${2:?'Json file path is empty.'}" 
                shift
            fi    
            ;;
        -p)  
            if [[ "$2" != -* ]]; then
                PASSWD="${2:?'Password is empty.'}" 
                shift
            fi
            ;;
        -r)  
            if [[ "$2" != -* ]]; then
                REGISTRY="${2:?'Registry url is empty.'}" 
                shift
            fi      
            ;;     
        -u) 
            if [[ "$2" != -* ]]; then 
                USERNAME="${2:?'Username is empty.'}" 
                shift
            fi        
            ;;
        -x) 
            if [[ "$2" != -* ]]; then
                REGISTRY_DIR="${2:?'Registry path is empty.'}" 
                shift
            fi
            ;;
        -*)
            if [[ -z "$2" || "$2" = -* ]]; then
                sub_options "$1"
            else
                sub_options "$1" "$2"
                shift
            fi    
            ;;
        *)
            echo -e "No such options ${1}. \nPlease read help manual." >&2 ; 
            exit 1
            ;;
        esac
        shift
    done
}

Usage(){
    cat <<EOF

${0##*/} is a registry clean script.

    Usage:
        -a             check, pull, delete, push images.
        -b             backup images.
        -s             upload images.     
        -c             clean local images that in current machine.
        -d             delete registry images. 
                       Please do not only run this option, 
                       except you already backup, and indeed want to 
                       clean all images in registry.
        -n <number>    restore <number> version tags of each image.
        -u             username.
        -h             help.
        -f             stdout file path, default is registry.json.
        -p             password.
        -r             registry url.
        -x             registry path.
    Example:
        [1] Backup one latest version image, and delete all images in registry.

            ${0##*/} -a -u admin -p admin -r 'http://127.0.0.1:5000' \ 
                    -x '/var/lib/docker/registry'

        [2] Backup three latest version image, and delete all images.

            ${0##*/} -a -u admin -p admin -n 3 -r 'http://127.0.0.1:5000' \ 
                        -x '/var/lib/docker/registry'

        [3] Backup one latest images, delete all images, and images list of registry stdout to a file.

            ${0##*/} -a -u admin -p admin -r 'http://127.0.0.1:5000' \ 
                    -x '/var/lib/docker/registry' -f '/tmp/registry.json'

        [4] Only execute backup, and images list of registry stdout to a file.

            ${0##*/} -b -u admin -p admin -r 'http://127.0.0.1:5000' \ 
                    -x '/var/lib/docker/registry' -f '/tmp/registry.json'    
        
        [5] Only clean local images.

            ${0##*/} -c 

EOF
}

all(){
    backup_images
    delete_registry
    push_image
}

main(){
    options "$@"

    
    test -n "$REGISTRY" || { echo "Registry url is empty." >&2 ; exit; }
    # test -n "$USERNAME" || { echo "Username is empty."; exit; }
    # test -n "$PASSWD" || { echo "Password is empty."; exit; }

    check_images

    test $FLAG_B -eq 0 && backup_images
    


    test -n "$REGISTRY_DIR" || { echo "Registry path is empty." >&2 ; exit; }

    
    test $FLAG_A -eq 0 && all && exit
    test $FLAG_D -eq 0 && delete_registry
    test $FLAG_S -eq 0 && push_image

    # test $FLAG_C -eq 0 && clean_local_images
}
main "$@"
exit
