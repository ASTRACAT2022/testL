#!/usr/bin/env bash
set -euo pipefail

# ------------------ 1. Проверяем, что Go уже есть ------------------
if ! command -v go &>/dev/null; then
    echo "❌  Go не найден. Установите Go вручную и перезапустите скрипт." >&2
    exit 1
fi
echo "✅  Найден Go: $(go version)"

# ------------------ 2. Собираем проект -----------------------------
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"   # папка, где лежит скрипт
BINARY_PATH="/usr/local/bin/astracat-dns"

echo "📦  Скачиваем зависимости…"
cd "$PROJECT_DIR"
go mod download

echo "🔨  Компилируем…"
go build -trimpath -ldflags="-s -w" -o "$BINARY_PATH" cmd/main.go

# ------------------ 3. Создаём systemd-unit ------------------------
SERVICE_FILE="/etc/systemd/system/astracat-dns.service"
sudo tee "$SERVICE_FILE" >/dev/null <<'EOF'
[Unit]
Description=AstraCat DNS Resolver
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/astracat-dns
Restart=always
RestartSec=5
User=nobody
Group=nogroup
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/cache/astracat-dns

[Install]
WantedBy=multi-user.target
EOF

# ------------------ 4. Активируем сервис ---------------------------
sudo systemctl daemon-reload
sudo systemctl enable astracat-dns.service
sudo systemctl restart astracat-dns.service

# ------------------ 5. Проверяем статус ----------------------------
sleep 1
if systemctl is-active --quiet astracat-dns; then
    echo
    echo "🎉  AstraCat DNS Resolver запущен!"
    echo "   Проверить:  sudo systemctl status astracat-dns"
    echo "   Логи:       sudo journalctl -u astracat-dns -f"
else
    echo
    echo "⚠️  Сервис не поднялся. Смотри логи:"
    echo "   sudo journalctl -u astracat-dns -n 50 --no-pager"
    exit 1
fi
