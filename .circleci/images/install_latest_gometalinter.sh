GOMETALINTER_VERSION=$(curl -s https://api.github.com/repos/alecthomas/gometalinter/releases/latest | jq -r ".tag_name")
echo "Latest Gometalinter version: $GOMETALINTER_VERSION"
GOMETALINTER_URL="https://github.com/alecthomas/gometalinter/releases/download/${GOMETALINTER_VERSION}/gometalinter-${GOMETALINTER_VERSION##v}-linux-amd64.tar.gz"
echo "Downloading from $GOMETALINTER_URL"
sudo mkdir /opt/gometalinter
curl -L $GOMETALINTER_URL | sudo tar --strip-components=1 -C /opt/gometalinter -xzf -
