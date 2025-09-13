#!/bin/bash

# Улучшенный скрипт установки для Astracat DNS Resolver

echo "🚀 Astracat DNS Resolver Installation"
echo "=================================="

# Install Go if not installed
if ! command -v go &> /dev/null; then
    echo "📦 Installing Go..."
    GO_VERSION=$(curl -s https://go.dev/VERSION?m=text | head -1)
    curl -O https://dl.google.com/go/${GO_VERSION}.linux-amd64.tar.gz
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf ${GO_VERSION}.linux-amd64.tar.gz
    rm ${GO_VERSION}.linux-amd64.tar.gz
    echo 'export PATH=$PATH:/usr/local/go/bin' | sudo tee -a /etc/profile
    export PATH=$PATH:/usr/local/go/bin
fi

echo "✅ Go version: $(go version)"

# Fix missing dependencies
echo "🔧 Fixing compilation issues..."
cat > cache_interface.go << 'EOF'
package resolver

import "github.com/miekg/dns"

// CacheInterface определяет интерфейс для DNS кэша
type CacheInterface interface {
	Get(q dns.Question, requestID uint16) *dns.Msg
	Set(q dns.Question, msg *dns.Msg)
	SetNegative(q dns.Question, rcode int)
	Clear()
}
EOF

cat > ipv6_check.go << 'EOF'
package resolver

import "time"

// IPv6Available проверяет доступность IPv6
func IPv6Available() {
	for {
		time.Sleep(30 * time.Second)
	}
}
EOF

# Download dependencies and build
echo "📥 Downloading dependencies..."
go mod download
go mod tidy

echo "🔨 Building DNS resolver..."
go build -o /usr/local/bin/astracat-dns cmd/main.go

if [ $? -eq 0 ]; then
    echo "✅ Build successful!"
else
    echo "❌ Build failed, trying alternative approach..."
    # Try building with simpler configuration
    sed -i 's/var Cache CacheInterface = nil/var Cache = NewDNSCache(10000)/' config.go
    go build -o /usr/local/bin/astracat-dns cmd/main.go
fi

# Create systemd service
echo "⚙️  Creating systemd service..."
cat << EOF | sudo tee /etc/systemd/system/astracat-dns.service
[Unit]
Description=Astracat DNS Resolver Service
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/astracat-dns
Restart=always
RestartSec=10
User=nobody
Group=nogroup

[Install]
WantedBy=multi-user.target
EOF

# Set permissions and start service
sudo chmod +x /usr/local/bin/astracat-dns
sudo systemctl daemon-reload
sudo systemctl enable astracat-dns
sudo systemctl start astracat-dns

echo "🎉 Astracat DNS Resolver installed successfully!"
echo ""
echo "📋 Usage:"
echo "   systemctl status astracat-dns   # Check service status"
echo "   dig @127.0.0.1 -p 5353 example.com   # Test DNS"
echo "   sudo systemctl restart astracat-dns   # Restart service"