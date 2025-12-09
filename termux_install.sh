ARCH = $(uname -m)

if [ "$ARCH" = "aarch64" ] ; then
    BINARY = "go-payload-dumper-termux-arm64"
    echo "✓ Detected ARM64 architecture"
    elif [ "$ARCH" = "armv7l" ] | | [ "$ARCH" = "armv8l" ] ; then
        BINARY = "go-payload-dumper-termux-armv7"
        echo "✓ Detected ARMv7 architecture"
        else
        echo "✗ Unsupported architecture: $ARCH"
        exit 1
    fi

    # Install dependencies
    echo ""
    echo "Installing dependencies..."
    pkg update -y
    pkg install -y xz-utils wget

    # Download binary
    echo ""
    echo "Downloading Go Payload Dumper..."
    RELEASE_URL = "https://github.com/OhMyDitzzy/go-payload-dumper/releases/latest/download/${BINARY}.tar.gz"

    wget -q --show-progress "$RELEASE_URL" -O /tmp/${BINARY}.tar.gz

    if [ $? -ne 0 ] ; then
        echo "✗ Failed to download binary"
        exit 1
    fi

    # Extract and install
    echo ""
    echo "Installing..."
    tar -xzf /tmp/${BINARY}.tar.gz -C /tmp/
    mv /tmp/${BINARY} $PREFIX/bin/go-payload-dumper
    chmod +x $PREFIX/bin/go-payload-dumper
    rm /tmp/${BINARY}.tar.gz

    echo ""
    echo "✅ Installation complete!"
    echo ""
    echo "Usage: go-payload-dumper -payload < file > "
    echo "Help: go-payload-dumper -h"