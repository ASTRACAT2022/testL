#!/bin/bash

# Install Go if not installed
if ! command -v go &> /dev/null; then
    echo "Installing Go..."
    # Download latest Go (fixed URL: removed extra space)
    GO_VERSION=$(curl -s https://go.dev/VERSION?m=text)
    curl -O "https://dl.google.com/go/${GO_VERSION}.linux-amd64.tar.gz"
    # Extract and install
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf "${GO_VERSION}.linux-amd64.tar.gz"
    rm "${GO_VERSION}.linux-amd64.tar.gz"
    # Add to PATH
    echo 'export PATH=$PATH:/usr/local/go/bin' | sudo tee -a /etc/profile
    source /etc/profile
fi

# Download dependencies and build the DNS server
echo "Building AstraCat DNS resolver..."
go mod download
go build -o /usr/local/bin/astracat-dns cmd/main.go

# Create systemd service file with name "astracat-dns"
cat << EOF | sudo tee /etc/systemd/system/astracat-dns.service
[Unit]
Description=AstraCat DNS Resolver Service
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

echo "AstraCat DNS Resolver installed and started successfully"
echo "Check status with: systemctl status astracat-dns"
