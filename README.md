# VF Swiss

Бутиковый сайт сервиса ремонта швейцарских часов на Go + SQLite. Клиенты могут ознакомиться с услугами и оставить заявку; заявки сохраняются в SQLite и отправляются владельцу по email.

## Стек и структура

- Go 1.24+
- SQLite (`modernc.org/sqlite`)
- SMTP для email
- Ubuntu VPS, Nginx и systemd
- `cmd/server` — запуск приложения
- `internal/` — конфигурация, база данных, обработчики, middleware и бизнес-логика
- `web/templates` — HTML-шаблоны
- `web/static` — CSS, JavaScript и изображения
- `web/uploads` — загруженные клиентами файлы; не храните содержимое этой папки в Git
- `deploy/` — шаблоны Nginx и systemd

## Локальный запуск

Нужен Go 1.24 или новее.

```powershell
Copy-Item .env.example .env
```

Заполните `.env` локальными значениями, затем:

```powershell
go test ./...
go run ./cmd/server
```

Сайт будет доступен по адресу `http://localhost:8080`. Для email задайте SMTP-параметры. Для Turnstile задайте оба ключа (`TURNSTILE_SITE_KEY` и `TURNSTILE_SECRET_KEY`); если оба пусты, проверка Turnstile отключена.

## Переменные окружения

Список переменных и безопасные заглушки находятся в [.env.example](./.env.example). Реальные `.env`-файлы и секреты нельзя добавлять в Git, образ контейнера, логи или команды CI. Храните production-файл окружения только на сервере с правами `0600`.

Если секрет когда-либо попадал в коммит, терминальный лог, публичный issue или другой общий канал, считайте его раскрытым: отзовите/смените его у провайдера и обновите на сервере. Простого удаления из последнего коммита недостаточно.

## Деплой на Ubuntu VPS через SSH

Текущий вариант деплоя — Linux-бинарник + файлы сайта + systemd + Nginx. Сервер размещается в Timeweb Cloud; IP и домен ниже оставлены заполнителями, чтобы не публиковать инфраструктурные данные в репозитории. В панели Timeweb настройте DNS A-запись домена на IP сервера и при необходимости правила сетевого экрана.

### 1. Первоначальная подготовка сервера

Подключитесь с доверенного компьютера по SSH:

```powershell
$SERVER_HOST = "YOUR_SERVER_IP"
ssh "root@$SERVER_HOST"
```

На сервере (Ubuntu):

```bash
apt update
apt install -y nginx certbot python3-certbot-nginx
adduser --system --group --home /opt/vfswiss --no-create-home vfswiss
install -d -o root -g vfswiss -m 0750 /opt/vfswiss
install -d -o root -g vfswiss -m 0750 /opt/vfswiss/web /opt/vfswiss/web/templates /opt/vfswiss/web/static
install -d -o vfswiss -g vfswiss -m 0750 /opt/vfswiss/database /opt/vfswiss/web/uploads
exit
```

Не отключайте root-доступ, пока не настроите и не проверите вход по SSH для отдельного административного пользователя.

### 2. Сборка и копирование приложения

Из корня проекта в Windows PowerShell соберите Linux-бинарник. Для VPS ARM64 замените `amd64` на `arm64`.

```powershell
New-Item -ItemType Directory -Force .\dist | Out-Null
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -trimpath -ldflags="-s -w" -o .\dist\vfswiss-linux-amd64 .\cmd\server
Remove-Item Env:GOOS
Remove-Item Env:GOARCH
```

Задайте тот же `$SERVER_HOST`, затем скопируйте бинарник, шаблоны, статику и unit-файл:

```powershell
scp .\dist\vfswiss-linux-amd64 "root@${SERVER_HOST}:/opt/vfswiss/vfswiss"
scp -r .\web\templates .\web\static "root@${SERVER_HOST}:/opt/vfswiss/web/"
scp .\deploy\vfswiss.service "root@${SERVER_HOST}:/etc/systemd/system/vfswiss.service"
```

### 3. Секреты и права доступа

На сервере создайте `/opt/vfswiss/.env` через `nano` или `sudoedit`, используя [.env.example](./.env.example) как перечень параметров. Укажите production-домен, абсолютные пути и новые SMTP/Turnstile секреты. Не копируйте в репозиторий локальный `.env`.

```bash
nano /opt/vfswiss/.env
chown root:root /opt/vfswiss/.env
chmod 0600 /opt/vfswiss/.env
chown root:vfswiss /opt/vfswiss/vfswiss
chmod 0750 /opt/vfswiss/vfswiss
chown -R root:vfswiss /opt/vfswiss/web/templates /opt/vfswiss/web/static
chown -R vfswiss:vfswiss /opt/vfswiss/database /opt/vfswiss/web/uploads
```

Для production установите как минимум:

```env
PORT=8080
DB_PATH=/opt/vfswiss/database/app.db
UPLOAD_DIR=/opt/vfswiss/web/uploads
STATIC_DIR=/opt/vfswiss/web/static
BASE_URL=https://YOUR_DOMAIN
```

Укажите SMTP-параметры; Turnstile включайте только при наличии обеих переменных.

### 4. Nginx, HTTPS и запуск

Скопируйте конфигурацию Nginx, замените `example.ru` на ваш домен и включите сайт:

```powershell
scp .\deploy\nginx.conf "root@${SERVER_HOST}:/etc/nginx/sites-available/vfswiss"
ssh "root@$SERVER_HOST"
```

На сервере:

```bash
nano /etc/nginx/sites-available/vfswiss
ln -s /etc/nginx/sites-available/vfswiss /etc/nginx/sites-enabled/vfswiss
nginx -t
systemctl reload nginx
systemctl daemon-reload
systemctl enable --now vfswiss
certbot --nginx -d YOUR_DOMAIN
curl -fsS https://YOUR_DOMAIN/health
```

Откройте в сетевом экране только необходимые входящие порты: SSH (лучше ограничить доверенными IP), HTTP и HTTPS. После настройки проверьте `systemctl status vfswiss`, `journalctl -u vfswiss` и автоматическое продление сертификата Certbot.

### Обновление вручную

Повторите сборку и копирование бинарника/шаблонов/статики из шага 2, затем перезапустите сервис:

```powershell
ssh "root@$SERVER_HOST" "systemctl restart vfswiss"
```

Проверьте `https://YOUR_DOMAIN/health`. База данных и загруженные клиентами файлы остаются в `/opt/vfswiss/database` и `/opt/vfswiss/web/uploads`; включите их в отдельные регулярные резервные копии и проверьте восстановление.

## Предлагаемый CI/CD pipeline (не настроен)

Для автоматического деплоя можно перейти на Docker-образ и GitHub Actions:

1. На pull request запускать `go test ./...`, `go vet ./...` и сборку Linux-образа; не выдавать таким job доступ к production-секретам.
2. После merge в `main` собирать образ, тегировать его commit SHA и публиковать в GitHub Container Registry (GHCR) с `GITHUB_TOKEN`.
3. Деплой запускать отдельным job после успешной сборки, ограничить его веткой `main` и GitHub Environment с ручным подтверждением.
4. Подключаться по SSH отдельным непривилегированным deploy-пользователем с отдельным ключом в GitHub Environment secret. Не использовать `root`, не помещать приватный ключ в репозиторий и закрепить проверенный SSH host key; не доверять непроверенному `ssh-keyscan` во время каждого деплоя.
5. На сервере выполнять `docker compose pull` и `docker compose up -d`, затем проверять `/health`. При сбое сохранять предыдущий image digest для отката.
6. Хранить SQLite и uploads в постоянных томах вне контейнера; SMTP, Turnstile и другие секреты передавать на сервере через защищённый env-файл/secret store, не встраивать в image и не выводить в логи.

Это предложение, а не готовая настройка: Dockerfile, Compose-файл, GitHub Actions workflow и SSH-учётная запись здесь не создаются. Текущий `deploy/vfswiss.service` запускает бинарник напрямую через systemd.

## Резервное копирование

Согласуйте расписание и место хранения резервных копий вне VPS. SQLite-файл и пользовательские uploads содержат данные клиентов; ограничьте доступ к копиям и регулярно проверяйте восстановление. Не добавляйте резервные копии в GitHub.

## Проверка после деплоя

- `/` — главная
- `/request` — отправка заявки
- `/success` — подтверждение заявки
- `/health` — `{"status":"ok"}`
