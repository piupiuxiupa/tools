#!/usr/bin/env bash

FONT="\033[32;49;1m"
BACK="\033[39;49;0m"
WARNING="\033[33;49;1m[WARNING]\033[39;49;0m"
CHECK="\033[33;49;1m[CHECK]\033[39;49;0m"
BACKUP="\033[33;49;1m[BACKUP]\033[39;49;0m"
ERROR="\033[31m [ERROR] \033[0m"
OK="\033[32m [OK] \033[0m"

FLAG_A=1
FLAG_U=1
FLAG_S=1
FLAG_F=1
FLAG_C=1
FLAG_P=1
FLAG_H=1

DATE=

USERNAME="guest"

function pwd_conf {
	sed -i.bak -e 's/# minlen = 9/minlen = 8/g;
				   s/# dcredit = 1/dcredit = -1/g;
				   s/# ucredit = 1/ucredit = -1/g;
				   s/# lcredit = 1/lcredit = -1/g;
				   s/# ocredit = 1/ocredit = -1/g' /etc/security/pwquality.conf
}

function add_user {
	awk -v error="$ERROR" -v user="$USERNAME" -F: '$1==user \
			{ printf "%s %s has existed\n", error,$1 \
				> "/dev/stdout"; exit 1 }' /etc/passwd

	test $? -ne 0 && return

	useradd $USERNAME
	echo 'pAsSvv@31d' | passwd --stdin $USERNAME

	test $? -eq 0 && {
		echo -e "$OK passwd success"
	}
}

function add_sudo {
	test "$(grep "^${USERNAME}.*$ALL" /etc/sudoers)" && {
		echo -e "$ERROR $USERNAME already has sudo priv." 
		return
	}

	sed -i.bak '/^root.*ALL$/a '$USERNAME' ALL=(ALL)  NOPASSWD:ALL' /etc/sudoers

	echo -e "$OK Change sudoers success"

}

function ssh_conf {
	# sed -i 's/#PermitRootLogin yes/PermitRootLogin no/g' /etc/ssh/sshd_config
	sed -i.bak 's/PermitRootLogin yes/PermitRootLogin no/g' /etc/ssh/sshd_config
	# sed -i 's/#MaxAuthTries 6/MaxAuthTries 3/g' /etc/ssh/sshd_config
	# sed -i '/# Example of overriding settings/a AllowUsers guest@$IPADDR/24' /etc/ssh/sshd_config
	service sshd restart
	echo -e "${OK} Service sshd setting finished"
}

function login_lock {

	test "$(grep "^auth.*require.*deny.*unlock.*300$" /etc/pam.d/sshd)" && {
		echo -e "${ERROR} Login Fail lock has exist" >&2
		return
	}

	sed -i.bak '1a auth       required     pam_tally2.so deny=3 unlock_time=300 even_deny_root root_unlock_time=300' /etc/pam.d/sshd
}

function set_history {
	####
	test "$(grep -E "^HIST.*=(1000|2000)$" /etc/profile)" && sed -i.bak "s/^HISTSIZE=1000/HISTSIZE=3000/g" /etc/profile


	test "$(grep "add_history" /etc/profile)" && echo -e "${ERROR} History setting has exist" >&2 && return

	echo "######### add_history ##########" >>/etc/profile
	echo 'export HISTTIMEFORMAT="%F %T  `whoami` CMD:"' >>/etc/profile
	echo 'shopt -s histappend' >>/etc/profile
	echo 'export PROMPT_COMMAND="history -a"' >>/etc/profile
	echo '######### add_history ##########' >>/etc/profile
	echo -e "${OK} History setting done"
}


# firewall check
function backupconfig {
	BACKUPFILE="$1"
	TARGETFILE="$2"
	command tar -zvcf $BACKUPFILE $TARGETFILE

	test $? -ne 0 && {
		echo -e "$BACKUP Backup firewalld config $ERROR" >&2
		return
	}

	echo -e "$BACKUP Backup firewall config $OK backupfile:$BACKUPFILE"
}

function errorexit {
	systemctl stop firewalld

	test $? -ne 0 && {
		echo -e "$ERROE$FONT Stop firewalld failed and exit$BACK$ERROR" >&2
		exit 1
	}

	echo -e "$OK $FONT Stop firewalld Success and exit$BACK"
	exit
}

function startfirewall {
	systemctl start firewalld
	test $? -eq 0 && {
		echo -e "$CHECK$FONT Start firewalld$BACK$OK \n"
		return
	}

	echo -e "$CHECK$FONT Start firewalld$BACK$ERROR" >&2
	errorexit
}

function reloadfirewall {
	test $(/usr/bin/firewall-cmd --reload) = "success" && {
		echo -e "${FONT}reload firewalld$BACK $OK \n"
		return
	}

	echo -e "${FONT}reload firewalld$BACK $ERROR" >&2
	errorexit
}

function check_firewall_status {
	test $(command firewall-cmd --state 2>/dev/null) = "running" && {
		echo -e "$CHECK$FONT firewalld is running$BACK$OK \n"
		return
	}
	echo -e "$CHECK firewalld is not running,Now starting firewalld...."
	startfirewall
}

function check_enable {
	test $(systemctl is-enabled firewalld) = "enabled" && {
		echo -e "$CHECK$FONT firewalld onboot is enabled$BACK $OK"
		return
	}

	echo -e "$CHECK firewalld onboot is disabled,now enable firewalld onboot..."
	systemctl enable firewalld
	test $? -eq 0 && echo -e "$CHECK$FONT firewalld onboot enabled$BACK$OK \n" && return

	echo -e "$CHECK$FONT firewalld onboot enabled$BACK$ERROR" >&2
	exit
}

function add_ports {
	TYPE=$1
	PORTS=$2
	for i in ${PORTS[@]}; do
		if [ $(/usr/bin/firewall-cmd --permanent --add-port=$i/$TYPE) = "success" ]; then
			echo -e "Add port $i/$TYPE $OK"
		else
			echo -e "Add port $i/$TYPE $ERROR"
			errorexit
		fi
	done
}

function check_port {
	TCPPORTS=($(command ss -tnl | awk 'NR >1 {print $4}' | cut -d: -f 2 | sort -un | tr '\n' ' '))
	echo -e "$FONT[1]$BACK Add$FONT TCP$BACK ports to firewall:$FONT $TCPPORTS $BACK"
	UDPPORTS=($(command ss -unl | awk 'NR >1 {print $4}' | cut -d: -f 2 | sort -un | tr '\n' ' '))
	echo -e "$FONT[2]$BACK Add$FONT UDP$BACK ports to firewall:$FONT $UDPPORTS $BACK"
	echo -e "$FONT[3]$BACK Add$FONT TCP and UDP$BACK ports to firewall \n"
}

function firewalld_cmd {
	backupconfig "$HOME/firewalld.tar.gz" "/etc/firewalld"

	check_port

	test ${#TCPPORTS} -lt 1 && echo -e "$ERROR No listening TCP Ports...." >&2 || add_ports "tcp" "${TCPPORTS[@]}"
	test ${#UDPPORTS} -lt 1 && echo -e "$ERROR No listening UDP Ports...." >&2 && return || add_ports "udp" "${UDPPORTS[@]}"

	reloadfirewall
	check_enable
}

## temporarily unavailable
function iptables_cmd {
	backupconfig "$HOME/iptables.tar.gz" "/etc/sysconfig/iptables /etc/sysconfig/ip*tables-config"
}

function check_firewall {

	test $(type -P firewall-cmd) && {
		echo -e "$CHECK$FONT Firewalld Install$BACK$OK \n"
		echo -e "$FONT Start add TCP and UDP ports to Firewalld....$BACK"
		firewalld_cmd

		return
	} ||
		echo -e "$CHECK$ERROR Firewalld is Not Install$BACK"

	# Check iptables
	test $(type -P iptables) && {
		echo -e "$CHECK$FONT Iptables Install$BACK$OK \n"
		iptables_cmd
		return
	} ||
		echo -e "$CHECK$ERROR Iptables is Not Install$BACK"
}


function all {
	echo -e "$OK Start configure password toctic--------------------------------------"
	pwd_conf
	echo -e "$OK Add user $USERNAME---------------------------------------------------"
	add_user
	echo -e "$OK Add sudoers----------------------------------------------------------"
	add_sudo
	echo -e "$OK SSH config-----------------------------------------------------------"
	ssh_conf
	echo -e "$OK Login config---------------------------------------------------------"
	login_lock
	echo -e "$OK Set history----------------------------------------------------------"
	set_history
	
	check_firewall
}

Usage(){
	cat	<<EOF
${0##*/} for Linux security check and configure.
	options:
		-h			print Usage and exit.
		-a			execute all options.
		-u			add normal user and set a password, default username "guest",
					default password "pAsSvv@31d". Please after run to reset password.
		-s			set sudo priv for new user.
		-c			set ssh limits.
		-p			set passowrd conf.
		-f			check listening ports and set firewall conf.
		-t			set history record conf.

	Example:
		- ${0##*/} 
			Execute all options, like -a.
		- ${0##*/} -f
			Only modify firewall conf and start firewall.
		- ${0##*/} -u "guest" -s
			Create user guest and add sudo priv.
EOF
}

function options {
	# ARGS=`command getopt -o au::s::fcp -- "$@"`
	# eval set -- "$ARGS"
	# echo ${ARGS}
	# test -z "$ARGS" && FLAG_A=0 && return

	test $# -eq 0 && FLAG_A=0 && return

	while [ $# -ne 0 ]
	do
		case "$1" in
			-a)
				FLAG_A=0; break ;;
			-u)
				FLAG_U=0; shift
				case "$1" in
					-*|"")	continue;;
					*) USERNAME="$1"; shift ;;
				esac
				;;
			-s)
				FLAG_S=0; shift
				case "$1" in
					-*|"")	continue;;
					*) USERNAME="$1"; shift ;;
				esac
				;;

			-f)
				FLAG_F=0; shift;;
			-c)
				FLAG_C=0; shift;;
			-p)
				FLAG_P=0; shift;;
			-t)
				FLAG_H=0; shift;;
			--)
				shift; break ;;
			*)
				Usage; exit
		esac
	done
}


main() {
	options "$@"

	test $FLAG_A -eq 0 && all && return
	test $FLAG_U -eq 0 && add_user
	test $FLAG_S -eq 0 && add_sudo
	test $FLAG_F -eq 0 && check_firewall
	test $FLAG_C -eq 0 && ssh_conf
	test $FLAG_P -eq 0 && pwd_conf
	test $FLAG_H -eq 0 && set_history
	

}

main "$@"

echo -e "$OK END--------------------------------------------------------------"

exit

:<<"EOF"
	This is a script to enforce Linux security that includes creating a new normal user, 
	prohibiting root from logging in remotely, enabling the firewall 
	and setting ports policies, setting password complexity policies, 
	and limiting the number of login failures.
	Only CentOS are available or other similar systems.
EOF

