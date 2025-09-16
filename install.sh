#!/bin/bash

# Install Go if not installed
if ! command -v go &> /dev/null; then
    echo "Installing Go..."
    # Download latest Go
    curl -O https://dl.google.com/go/$(curl -s https://go.dev/VERSION?m=text).linux-amd64.tar.gz
    # Extract and install
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf go*.linux-amd64.tar.gz
    rm go*.linux-amd64.tar.gz
    # Add to PATH
    echo 'export PATH=$PATH:/usr/local/go/bin' | sudo tee -a /etc/profile
    source /etc/profile
fi

# Download dependencies and build the DNS server
echo "Building DNS resolver..."
go mod download
go build -o /usr/local/bin/dns-resolver cmd/main.go

# Create systemd service file
cat << EOF | sudo tee /etc/systemd/system/dns-resolver.service
[Unit]
Description=DNS Resolver Service
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/dns-resolver
Restart=always
User=nobody
Group=nogroup

[Install]
WantedBy=multi-user.target
EOF

# Set permissions
sudo chmod +x /usr/local/bin/dns-resolver

# Reload systemd and enable service
sudo systemctl daemon-reload
sudo systemctl enable dns-resolver
sudo systemctl start dns-resolver

echo "DNS Resolver installed and started successfully"
echo "Check status with: systemctl status dns-resolver"