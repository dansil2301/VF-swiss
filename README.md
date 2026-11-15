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

## Production-деплой на Timeweb Cloud

Production уже работает без Docker: Ubuntu 26.04, x86_64, `systemd`-сервис `vfswiss` от пользователя `vfswiss`, Nginx проксирует приложение на `127.0.0.1:8080`, домен — `vf-swiss.ru`. Docker на сервере не установлен и для этого деплоя не нужен.

Файл [`.github/workflows/deploy.yml`](./.github/workflows/deploy.yml) запускает `go test ./...` и `go vet ./...`, собирает Linux/amd64-бинарник и при включённом переключателе `DEPLOY_ENABLED` загружает его, шаблоны и статику по SSH. Деплой запускает `systemd`; после перезапуска workflow проверяет `https://vf-swiss.ru/health`.

Деплой не копирует и не изменяет `/opt/vfswiss/.env`, базу `/opt/vfswiss/database/app.db`, `/opt/vfswiss/web/uploads`, конфигурацию Nginx или unit-файл systemd. Архив проверяется перед распаковкой; серверный helper откатывает бинарник и сайт, если сервис не запускается. Не добавляйте секреты, базу или пользовательские файлы в GitHub.

### Однократная подготовка сервера

Сгенерируйте отдельный SSH-ключ без парольной фразы на компьютере, где доступен OpenSSH:

```powershell
ssh-keygen -t ed25519 -f "$env:USERPROFILE\.ssh\vfswiss-actions" -C "github-actions-vfswiss"
```

Оставьте парольную фразу пустой. **Приватный файл `vfswiss-actions` не отправляйте в чат и не добавляйте в репозиторий.** Передайте на сервер только публичный ключ `vfswiss-actions.pub`.

Скопируйте helper на сервер с доверенного компьютера:

```powershell
$SERVER_HOST = "YOUR_SERVER_IP"
scp .\deploy\vfswiss-deploy.sh "root@${SERVER_HOST}:/tmp/vfswiss-deploy.sh"
ssh "root@$SERVER_HOST"
```

На сервере однократно выполните:

```bash
useradd --create-home --home-dir /var/lib/vfswiss-deploy --shell /bin/bash vfswiss-deploy
install -d -o vfswiss-deploy -g vfswiss-deploy -m 0700 /var/lib/vfswiss-deploy/incoming
install -d -o vfswiss-deploy -g vfswiss-deploy -m 0700 /var/lib/vfswiss-deploy/.ssh
install -o root -g root -m 0750 /tmp/vfswiss-deploy.sh /usr/local/sbin/vfswiss-deploy
printf '%s\n' 'vfswiss-deploy ALL=(root) NOPASSWD: /usr/local/sbin/vfswiss-deploy' > /etc/sudoers.d/vfswiss-deploy
chmod 0440 /etc/sudoers.d/vfswiss-deploy
visudo -cf /etc/sudoers.d/vfswiss-deploy
```

Добавьте содержимое публичного `.pub`-файла в `/var/lib/vfswiss-deploy/.ssh/authorized_keys`, поставив `restrict` перед `ssh-ed25519`, например:

```text
restrict ssh-ed25519 AAAA... github-actions-vfswiss
```

Затем задайте владельца и права:

```bash
chown vfswiss-deploy:vfswiss-deploy /var/lib/vfswiss-deploy/.ssh/authorized_keys
chmod 0600 /var/lib/vfswiss-deploy/.ssh/authorized_keys
```

У deploy-пользователя нет root-доступа по SSH; `sudo` разрешает ему только root-owned helper без аргументов. Опция `restrict` запрещает SSH port/agent/X11 forwarding и PTY для этого ключа. Проверьте sudoers командой `visudo -cf` до включения pipeline.

При проверке сервера обнаружено, что SQLite-файл имеет права `0644`; ограничьте чтение файла пользователем приложения:

```bash
chmod 0600 /opt/vfswiss/database/app.db
```

В Nginx также настроен `alias` на каталог uploads. Фото из заявок могут содержать личные данные: убедитесь, что этот URL не открывает их посторонним. Pipeline этот alias не меняет.

### GitHub Actions: секреты и включение автодеплоя

Доступный GitHub-интерфейс позволяет проверять workflow, но не записывать Actions secrets. Добавьте их самостоятельно в `dansil2301/VF-swiss` → **Settings → Secrets and variables → Actions → New repository secret**:

| Secret | Значение |
|---|---|
| `DEPLOY_HOST` | Публичный IP или SSH hostname сервера |
| `DEPLOY_PORT` | SSH-порт, обычно `22` |
| `DEPLOY_USER` | `vfswiss-deploy` |
| `DEPLOY_SSH_KEY` | Содержимое приватного `vfswiss-actions` (не `.pub`) |
| `DEPLOY_KNOWN_HOSTS` | Проверенная запись host key для `DEPLOY_HOST` и порта |

Для `DEPLOY_KNOWN_HOSTS` сначала получите fingerprint host key на сервере (`ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub`), затем сверьте его с результатом `ssh-keyscan -t ed25519 -p 22 YOUR_SERVER_IP` на своём компьютере. Только после совпадения добавляйте строку `ssh-keyscan` в secret. Не полагайтесь на непроверенный ключ и не запускайте `ssh-keyscan` без сверки при каждом деплое.

После установки серверного helper и проверки SSH создайте в **Settings → Secrets and variables → Actions → Variables** переменную `DEPLOY_ENABLED` со значением `true`. До этого workflow выполняет тесты, но пропускает production deploy. После включения каждый push в `main` автоматически тестирует, собирает и выкатывает приложение. Первый запуск текущей версии можно вручную запустить через **Actions → Test and deploy → Run workflow**.

При ошибке откройте Actions run; на сервере доступны `systemctl status vfswiss` и `journalctl -u vfswiss`. База и uploads сохраняются на месте. До включения автоматического деплоя настройте резервное копирование `/opt/vfswiss/database` и `/opt/vfswiss/web/uploads` вне VPS.

## Резервное копирование

Согласуйте расписание и место хранения резервных копий вне VPS. SQLite-файл и пользовательские uploads содержат данные клиентов; ограничьте доступ к копиям и регулярно проверяйте восстановление. Не добавляйте резервные копии в GitHub.

## Проверка после деплоя

- `/` — главная
- `/request` — отправка заявки
- `/success` — подтверждение заявки
- `/health` — `{"status":"ok"}`
