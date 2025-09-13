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

# Build the Astracat DNS server
echo "Building Astracat DNS..."
go mod tidy
go build -o /usr/local/bin/astracat-dns ./cmd

# Create systemd service file
cat << EOF | sudo tee /etc/systemd/system/astracat-dns.service
[Unit]
Description=Astracat DNS Resolver Service
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/astracat-dns
Restart=always
User=nobody
Group=nogroup

[Install]
WantedBy=multi-user.target
EOF

# Set permissions
sudo chmod +x /usr/local/bin/astracat-dns

# Reload systemd and enable service
sudo systemctl daemon-reload
sudo systemctl enable astracat-dns
sudo systemctl start astracat-dns

echo "Astracat DNS Resolver installed and started successfully"
echo "Check status with: systemctl status astracat-dns"
