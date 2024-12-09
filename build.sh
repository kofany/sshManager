#!/bin/bash

# Set the output directory
OUTPUT_DIR="./build"
mkdir -p "$OUTPUT_DIR"

# Platforms to build for
platforms=(
  "windows/amd64"
  "darwin/amd64"
  "darwin/arm64"
  "linux/amd64"
  "linux/arm64"
)

# Binary name
BINARY_NAME="sshm"

# Build binaries
for platform in "${platforms[@]}"; do
  IFS="/" read -r GOOS GOARCH <<< "$platform"
  
  # Set output file name
  output_name="$OUTPUT_DIR/${BINARY_NAME}_${GOOS}_${GOARCH}"
  if [ "$GOOS" == "windows" ]; then
    output_name+=".exe"
  fi
  
  echo "Building for $GOOS/$GOARCH..."
  
  # Build the binary
  GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED=0 go build -ldflags="-extldflags '-static'" -o "$output_name" ./cmd/sshm/main.go
  
  if [ $? -ne 0 ]; then
    echo "Failed to build for $GOOS/$GOARCH"
    exit 1
  fi
done

echo "Builds completed. Binaries are in $OUTPUT_DIR"
