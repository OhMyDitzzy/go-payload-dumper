# Installing on Termux

This guide will help you install and use Go Payload Dumper on Android via Termux.

# Manual Installation
## Step 1: Install Dependencies
```bash
pkg update
pkg install -y golang git xz-utils
```

## Step 2: Clone and Build
```bash
# Clone repository
git clone https://github.com/OhMyDitzzy/go-payload-dumper.git
cd go-payload-dumper

# Install protoc
pkg install -y protobuf

# Install protoc-gen-go
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
export PATH="$PATH:$(go env GOPATH)/bin"

# Generate protobuf code
protoc --go_out=. --go_opt=paths=source_relative protos/update_metadata.proto

# Build
go build -o go-payload-dumper ./cmd/dumper

# Install to PATH
mv go-payload-dumper $PREFIX/bin/
```

## Step 3: Verify Installation
```bash
go-payload-dumper -version
```

For usage, See [README](README).