

---

# Astracat DNS Resolver

Astracat DNS Resolver — быстрый и безопасный рекурсивный DNS-сервер с поддержкой DNSSEC.

## Установка

### 1. Клонируем репозиторий

```bash
git clone https://github.com/ASTRACAT2022/Astracat-DNS-Resolver.git
cd Astracat-DNS-Resolver
```

---

### 2. Делаем скрипт установки исполняемым

```bash
chmod +x install.sh
```

---

### 3. Запускаем установку

```bash
sudo ./install.sh
```

Скрипт автоматически:

1. Установит Go (если не установлен)
2. Создаст структуру папок `cmd` и `resolver`
3. Соберёт проект и создаст бинарник `/usr/local/bin/astracat-dns`
4. Создаст systemd-сервис `astracat-dns` и запустит его

---

### 4. Проверка статуса сервиса

```bash
systemctl status astracat-dns
```

Вы должны увидеть, что сервис активен (`active (running)`).

---

### 5. Запуск и остановка сервиса

```bash
sudo systemctl start astracat-dns
sudo systemctl stop astracat-dns
sudo systemctl restart astracat-dns
```

---

### 6. Тестирование DNS

Для проверки работы резолвера:

```bash
dig @127.0.0.1 -p 5353 example.com
dig +dnssec @127.0.0.1 -p 5353 dnssec-failed.org
```

---

### 7. Требования

* Linux (Debian/Ubuntu, RHEL/CentOS и др.)
* Bash
* Сеть с доступом к интернет для скачивания Go и зависимостей

---
