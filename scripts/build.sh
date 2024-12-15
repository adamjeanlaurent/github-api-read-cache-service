go mod tidy

mkdir -p /bin

# build service for different CPU and OS flavors
echo "Building Linux AMD64"
GOOS=linux GOARCH=amd64 go build -o bin/server-linux-amd

echo "Building Linux ARM64"
GOOS=linux GOARCH=arm64 go build -o bin/server-linux-arm

echo "Building Windows AMD64"
GOOS=windows GOARCH=amd64 go build -o bin/server-windows-amd.exe

echo "Building Windows ARM64"
GOOS=windows GOARCH=amd64 go build -o bin/server-windows-arm.exe

echo "Building Mac AMD64"
GOOS=darwin GOARCH=amd64 go build -o bin/server-mac-amd

echo "Building Mac ARM64"
GOOS=darwin GOARCH=arm64 go build -o bin/server-mac-arm
