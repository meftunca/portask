#!/bin/bash

echo "🚀 Starting Portask server..."

cd /Users/mapletechnologies/go-workspace/src/github.com/meftunca/portask

# Build the server first
echo "🔨 Building server..."
go build -o ./portask-server ./cmd/server
if [ $? -ne 0 ]; then
    echo "❌ Build failed!"
    exit 1
fi

echo "✅ Build successful, starting server..."

# Start the server
./portask-server
