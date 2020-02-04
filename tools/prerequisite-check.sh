#!/bin/bash

if [ "$SKIP_PREREQUISITE_CHECK" ] ; then
	echo "Skip prerequisite_check."
	exit 0
fi

if [ ! "$SGX_MODE" ] ; then
	SGX_MODE="HW"
fi

REQUIRED_GO_VERSION=1.11
REQUIRED_SGXSDK_VERSION=2.3.101

show_os_info() {
	echo "OS: $(. /etc/os-release ; echo $PRETTY_NAME)"
}

compare_version() {
	local ver1=$(echo "$1" | awk -F. '{ printf("%d%03d%03d%05d\n", $1,$2,$3,$4); }';)
	local ver2=$(echo "$2" | awk -F. '{ printf("%d%03d%03d%05d\n", $1,$2,$3,$4); }';)

	if [ "$ver1" -lt "$ver2" ] ; then
		return 1
	else
		return 0
	fi
}

check_go_version() {
	if [[ "$(go version)" =~ go([0-9]+\.[0-9]+\.?[0-9]*) ]] ; then
		local goversion=${BASH_REMATCH[1]}
	else
		echo "Failed to parse go version. Please install it."
		return 1
	fi

	if ! compare_version "$goversion" "$REQUIRED_GO_VERSION" ; then
		echo "Your Go (version $goversion) is older than required version ($REQUIRED_GO_VERSION), so please update it."
		return 1
	fi

	echo "Go version: $goversion"
}

check_sgxsdk_version() {
	local sgxsdk_version=$(grep ^Version: $SGX_SDK/pkgconfig/libsgx_urts.pc | awk '{print $2}')
	if [ ! "$sgxsdk_version" ] ; then
		echo "Failed to parse SGX SDK version from $SGX_SDK/pkgconfig/libsgx_urts.pc. Please make sure that SGX SDK is properly installed."
		return 1
	fi

	if ! compare_version $sgxsdk_version $REQUIRED_SGXSDK_VERSION ; then
		echo "Your SGX SDK (version $sgxsdk_version) is older than required version ($REQUIRED_SGXSDK_VERSION), so please update it."
		return 1
	fi

	echo "Intel SGX SDK version: $sgxsdk_version"
}

check_sgx_support() {
	if [ "$SGX_MODE" != "HW" ] ; then
		return 0
	fi

	if make -s --no-print-directory -C tools/sgx-capability check ; then
		return 0
	else
		echo "Intel SGX is not available. Please set environment variable SGX_MODE=SIM."
		return 1
	fi
}

get_sgx_psw_version() {
	local distro="$(. /etc/os-release ; echo $PRETTY_NAME)"
	local version=

	if [[ "$distro" =~ "Ubuntu" ]] ; then
		version=$(dpkg -s libsgx-enclave-common 2> /dev/null | grep '^Version:' | awk '{print $2}')
	elif [[ "$distro" =~ "Red Hat Enterprise Linux" ]] ; then
		version=$(rpm -qi libsgx-enclave-common 2> /dev/null | grep '^Version' | awk '{print $3}')
	elif [[ "$distro" =~ "CentOS" ]] ; then
		version=$(rpm -qi libsgx-enclave-common 2> /dev/null | grep '^Version' | awk '{print $3}')
	else
		echo "Your Linux distribution '$os' is not supported now." >&2
		return 1
	fi

	if [ "$version" ] ; then
		echo "$version"
		return 0
	else
		echo "Failed to get SGX PSW version." >&2
		return 1
	fi
}

check_sgxpsw_version() {
	if [ "$SGX_MODE" != "HW" ] ; then
		return 0
	fi

	local version="$(get_sgx_psw_version)"
	if [ "$version" ] ; then
		echo "Intel SGX PSW version: $version"
		return 0
	else
		echo "Intel SGX PSW is not installed in your system. Please install it or set environment variable SGX_MODE=SIM."
		return 1
	fi
}

show_os_info

if ! check_go_version ; then
	exit 1
fi

if [ ! "$SGX_SDK" ] ; then
	echo "Environment variable SGX_SDK not set, please make sure to source the environment file given by SGX SDK."
	exit 1
fi

if [ "$SGX_MODE" != "HW" ] ; then
	echo "SGX is running in simulation mode."
fi

if ! check_sgxsdk_version ; then
	exit 1
fi

if ! check_sgx_support ; then
	exit 1
fi

if ! check_sgxpsw_version ; then
	exit 1
fi

echo "Enviroment check passed."
