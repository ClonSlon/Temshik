# Cloudflare Tunnel for qerix.uk

## 1. Локальный сайт
Сайт уже запускается так:

```bash
cd site
python3 -m http.server 4173
```

## 2. Установка cloudflared
Если не установлен:

```bash
brew install cloudflared
```

## 3. Авторизация в Cloudflare
```bash
cloudflared tunnel login
```

Откроется браузер. Нужно выбрать зону `qerix.uk`.

## 4. Создание tunnel
```bash
cloudflared tunnel create qerix-landing
```

Команда вернёт `TUNNEL_ID` и создаст credentials json в `~/.cloudflared/`.

## 5. Конфиг
Скопировать шаблон:

```bash
cp site/cloudflared.example.yml ~/.cloudflared/config.yml
```

Потом подставить:
- `YOUR_TUNNEL_ID`
- путь к json credentials

## 6. DNS маршруты
```bash
cloudflared tunnel route dns qerix-landing qerix.uk
cloudflared tunnel route dns qerix-landing www.qerix.uk
```

## 7. Запуск tunnel
```bash
cloudflared tunnel run qerix-landing
```

## 8. Автозапуск на macOS
```bash
brew services start cloudflared
```

Если нужен ручной режим без сервиса:

```bash
cloudflared tunnel run qerix-landing
```

## Примечание
Сейчас `qerix.uk` отвечает Cloudflare ошибкой 523. Это ожидаемо, пока не привязан рабочий origin через tunnel или другой хостинг.
