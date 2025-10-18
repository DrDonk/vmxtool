#!/bin/bash
VERSION=$(<VERSION)
BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

echo "Building vmxtool version $VERSION"
echo "Build date: $BUILD_DATE"
echo "Commit: $COMMIT"

LDFLAGS="-X main.Version=$VERSION -X main.BuildDate=$BUILD_DATE -X main.Commit=$COMMIT"

mkdir -p build
cp -v README.md ./build
cp -v LICENSE ./build
cp -v sample.vmx ./build

# AMD64 builds
echo "Building AMD64 versions..."
GOOS=windows GOARCH=amd64 go build -ldflags="$LDFLAGS" -o build/windows/amd64/vmxtool.exe vmxtool.go
GOOS=linux GOARCH=amd64 go build -ldflags="$LDFLAGS" -o build/linux/amd64/vmxtool vmxtool.go
GOOS=darwin GOARCH=amd64 go build -ldflags="$LDFLAGS" -o build/darwin/amd64/vmxtool vmxtool.go

# ARM64 builds
echo "Building ARM64 versions..."
GOOS=windows GOARCH=arm64 go build -ldflags="$LDFLAGS" -o build/windows/arm64/vmxtool.exe vmxtool.go
GOOS=linux GOARCH=arm64 go build -ldflags="$LDFLAGS" -o build/linux/arm64/vmxtool vmxtool.go
GOOS=darwin GOARCH=arm64 go build -ldflags="$LDFLAGS" -o build/darwin/arm64/vmxtool vmxtool.go

# Build distribution zip file
rm -vf ./dist/vmxtool-$VERSION.zip
rm -vrf ./dist/vmxtool-$VERSION.sha256
7z a ./dist/vmxtool-$VERSION.zip ./build/*
shasum -a 256 ./dist/vmxtool-$VERSION.zip > ./dist/vmxtool-$VERSION.sha256

echo "Build complete!"
