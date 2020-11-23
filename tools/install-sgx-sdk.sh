#!/bin/sh

trap 'rm -rf "$TMP_DIR"' EXIT

. /etc/os-release

if [ "$ID-$VERSION_ID" = "ubuntu-18.04" ]; then
    DISTRO="ubuntu18.04-server"
    apt-get -y install build-essential python
elif [ "$ID-$VERSION_ID" = "ubuntu-20.04" ]; then
    DISTRO="ubuntu20.04-server"
    apt-get -y install build-essential python-is-python3
else
    echo "This platform is not supported"
    exit 1
fi

URL="https://download.01.org/intel-sgx/sgx-linux/2.12/distro/$DISTRO/sgx_linux_x64_sdk_2.12.100.3.bin"
TMP_DIR=`mktemp -d`
PACKAGE="$TMP_DIR/sgx_linux_x64_sdk.bin"
curl -s "$URL" -o "$PACKAGE"
if [ $? -ne 0 ]; then
    echo "Could not download the installer"
    exit 1
fi
chmod +x "$PACKAGE"
echo -e "no\n/opt/intel" | "$PACKAGE"
