#!/usr/bin/env bash

: <<notes
date: 2023-04-24
version: v1
notes

## default values
output_file_name=""
search_path="$PWD"
flag='M'
depth=2
start_date=$(date -d 'yesterday' +%Y%m%d)
end_date=$(date -d 'now' +%Y%m%d)
mode=0 # 1 --- quiet; 0 --- show result

Usage() {
    cat <<EOF
        Calc generated files between two dates used capacities in disk.
        -p | --path | --path=        Path that you want to check. The path not use / to perform better.
             --unit | --unit=        Replace -g or -m. Select values M or G.
        -f                           Output results to a file.
        -g | -G                      Using G unit to calc.
        -m | -M                      Using M unit to calc.
        -d                           Date that you want to check.  Example: sh ${0##*/} -d 20230101 20230102.
EOF
}

## Find files that generated between start_date and end_date
function find_files() {
    find $search_path -newermt "$start_date" ! -newermt "$end_date" -type f -ls 2>>/dev/null
}

function output_files_path() {
    if test -e $output_file_name; then
        mv ${output_file_name} ${output_file_name}_$(stat -c %Y $output_file_name)
    fi
    if test $mode -eq 1; then
        find_files >${output_file_name}
    else
        find_files | tee $output_file_name
    fi
    echo "The output file name is $output_file_name"
}

function count_c() {
    for i in $files_size_set; do
        let sum+=$i
        # shift
    done
}

## Calc files totol size
function calc_size() {
    if test -z $output_file_name; then
        if test $mode -eq 0; then
            find_files &
            find_res=$(find_files &)
            # echo "$find_res"
            wait $!
            files_size_set=$(echo "$find_res" | awk '{print $7}')

            count_c
        else
            files_size_set=$(find_files | awk '{print $7}')

            count_c
        fi
    else
        output_files_path

        files_size_set=$(cat $output_file_name | awk '{print $7}')

        count_c
    fi

    ## Transfer human format
    ## echo "111111111/(1024^3)"|bc
    # echo "$(find ./ -newermt "$1" ! -newermt "$2" -type f -ls | awk '{print $7}'| tr '\n' '+')0"| bc| xargs -I {} expr {} / 1024 / 1024 / 1024 | xargs -I {}  echo "{}G"
    # sed -n 's/\(.*\)\(.\)/\1/g'   ----- del or replace the last char.
    if which bc &>>/dev/null; then
        res=$(printf "%.2f" $(echo "scale=2;${sum}/1024^${depth}" | bc 2>>/dev/null))
    else
        res=$(echo "$sum $depth" | awk '{printf ("%.2f",$1/1024^$2)}')
    fi
    case $flag in
    k | K) res_info="${res}K" ;;
    m | M) res_info="${res}M" ;;
    g | G) res_info="${res}G" ;;
    t | T) res_info="${res}T" ;;
    esac

}

main() {
    echo "Calculating, please wait a moment ..."
    calc_size
    echo -e "Checked path: $search_path.\nBetween the $start_date to $end_date generated files calc result is ${res_info}."
}

[ $# -eq 0 ] && {
    # echo "Please input -h to read Usage."
    main
    exit
}

# echo "$*"

while test $# -ne 0; do
    case ${1} in
    -f)
        if test -z "$2" || [[ "$2" = -* ]]; then
            output_file_name="output_files.txt"
            shift
        else
            output_file_name="$2"
            shift 2
        # else
        #     echo "Please input a correct file name ..."
        #     exit
        fi
        ;;
    -p | --path | --path=*)
        if test -z "$2"; then
            echo "Please input a correct path..."
            exit
        fi
        case "$1" in
        -p)
            search_path="$2"
            shift 2
            ;;
        --path)
            search_path="$2"
            shift 2
            ;;
        --path=*)
            search_path="${1#--path=}"
            shift
            ;;
        *)
            echo "Input a path to search files."
            exit
            ;;
        esac
        ;;
    -d)
        shift
        if test -z $1 && test -z $2; then
            echo "Please input two dates, format: year-month-days or like 20230101..."
            echo "Example: started time 20230101, ended time 20230102"
            exit
        fi
        if [[ "$1" =~ [0-9]{8} ]] && [[ "$2" =~ [0-9]{8} ]]; then
            start_date=$1
            end_date=$2
            shift 2
        else
            echo "Please input two dates, format: year-month-days or like 20230101..."
            echo "Example: started time 20230101, ended time 20230102"
            exit
        fi
        ;;
    --unit | --unit=*)
        if test "${1#--unit=}" != "$1" -a "${1#--unit=}" = "M" -o "${1#--unit=}" = "G"; then
            flag="${1#--unit=}"
            shift
        elif test "$1" = "--unit" -a "${2}" = "M" -o "${2}" = "G"; then
            flag=$2
            shift 2
        else
            echo "Unit must is M or G !"
            exit
        fi
        case $flag in
        M | m) depth=2 ;;
        G | g) depth=3 ;;
        esac
        ;;
    -q)
        mode=1
        shift
        ;;

    -k | -K | -m | -M | -g | -G | -t | -T)
        flag=${1#-}
        case $flag in
        k | K) depth=1 ;;

        m | M) depth=2 ;;

        g | G) depth=3 ;;

        t | T) depth=4 ;;
        esac
        shift
        ;;
    *)
        Usage
        exit
        ;;
    esac
done

# echo "$start_date  $end_date"
# echo "flag is $flag."

main
