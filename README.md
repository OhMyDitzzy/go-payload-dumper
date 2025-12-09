# Go Payload Dumper
A high-performance Android OTA payload dumper written in Go. This tool extracts partition images from Android OTA update packages, supporting both full.

## Overview
Android OTA (Over-The-Air) updates are distributed as payload files containing compressed partition images. This tool unpacks those payload files and extracts the individual partition images (system, vendor, boot, etc.) that you can then flash to your device or analyze.
The tool is built with [Go](https://go.dev) for superior performance and cross-platform compatibility. It handles various compression formats including XZ, Bzip2, Zstandard, and supports complex operations like binary diffing for incremental updates. Whether you're working with local files, remote URLs, or ZIP archives, this dumper handles it all seamlessly.

## Features
### Comprehensive Format Support
- Extracts from local payload.bin files
- Downloads and extracts from remote URLs (HTTP/HTTPS)
- Automatically detects and extracts from ZIP archives
- Supports both full OTA and differential/incremental OTA packages
### Advanced Compression & Operations
- REPLACE, REPLACE_BZ, REPLACE_XZ, ZSTD decompression
- BSDIFF and BROTLI_BSDIFF binary patching for incremental updates
- SOURCE_COPY operations for efficient data transfer
- ZERO operations for partition initialization
- SHA256 hash verification for data integrity

## Installation
You'll need Go 1.21 or higher and some system dependencies installed on your machine.
### Install System Dependencies
The tool requires xz-utils for handling XZ-compressed data, which is commonly used in Android OTA packages.
- Ubuntu/Debian:
```
sudo apt-get update
sudo apt-get install xz-utils
```
- macOS:
```
brew install xz
```
- Arch Linux:
```bash
sudo pacman -S xz
```
- Windows:
Install WSL2 (Windows Subsystem for Linux) and follow the Ubuntu instructions, or download xz-utils from https://tukaani.org/xz/
### Install Protocol Buffers Compiler
The project uses Protocol Buffers to parse Android's update metadata format.
```bash
# Install protoc-gen-go
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

# Make sure it's in your PATH
export PATH="$PATH:$(go env GOPATH)/bin"
```

### Build the tool
```bash
# Clone the repository
git clone https://github.com/OhMyDitzzy/go-payload-dumper.git
cd go-payload-dumper

# Generate Protocol Buffer code from the .proto file
protoc --go_out=. --go_opt=paths=source_relative protos/update_metadata.proto

# Download dependencies
go mod download

# Build the binary
go build -o go-payload-dumper ./cmd/dumper

# The binary is now ready to use
./go-payload-dumper -version
If you want to install it system-wide:
go install ./cmd/dumper
# Now you can run it from anywhere as 'dumper'
```

## Usage Examples
Basic Extraction
The simplest use case - extract all partitions from a local payload file:
```bash
./go-payload-dumper -payload payload.bin
```
This will create an output directory and extract all partition images (system.img, vendor.img, boot.img, etc.) into it. Or, If you want to extract a specific partition, use:
```bash
./go-payload-dumper -images boot -payload payload.bin
# Can: -images boot,vendor etc...
```
### Extract from Remote URL
No need to download large OTA files manually. Point directly to the URL:
```bash
./go-payload-dumper -payload https://dl.google.com/dl/android/aosp/bluejay-ota-ap1a.240505.005-3c1c6c2e.zip
```
The tool will download it, detect if it's a ZIP, extract payload.bin, and dump all partitions. Perfect for automated workflows.

## Troubleshooting
### "Invalid magic header" error
The file you're trying to extract isn't a valid OTA payload. Make sure:
- You're pointing to the correct file (payload.bin or a ZIP containing it)
- The file isn't corrupted (check file size, try re-downloading)
- You're not trying to extract a different type of Android image (like a fastboot image)

### "SOURCE_COPY requires old file for differential OTA" error
This means you're trying to extract an incremental OTA without providing the base images. You need:
1. The original/base partition images in a directory
2. Use the -diff flag
3. Point to the base images directory with -old

### "xz: unsupported filter count" error
The XZ library fallback to system xz command. Make sure xz-utils is installed:
```bash
# Check if xz is available
which xz

# Install if missing (Ubuntu/Debian)
sudo apt-get install xz-utils
```

### "payload.bin not found in zip" error
The ZIP file you provided doesn't contain a payload.bin file. Some things to check:
- Make sure it's an actual OTA update ZIP (not a ROM ZIP or fastboot package)
- Try extracting the ZIP manually to verify its contents
- Some OTA packages might use different internal structures

### Out of memory errors
Large payloads (especially system partitions) can be memory-intensive. If you run out of RAM:
- Extract partitions one at a time using -images
- Close other applications to free up memory
- Use a machine with more RAM
- Consider extracting on a system with swap space enabled

### Hash mismatch errors
If you see "data hash mismatch" errors, the payload file is corrupted:
- Re-download the OTA package
- Verify the checksum if one is provided by the source
- Check your disk for errors (corrupted storage can cause this)

## Contributing
Found a bug? Want to add a feature? Contributions are welcome!
The codebase is intentionally kept simple. Fork the repository, make your changes, and submit a pull request.

## License
[MIT License](LICENSE) - feel free to use this in your own projects, commercial or otherwise.