# mittodatura sdk

клиенты и примеры интеграции для b2b api шлюза цифровых товаров mittodatura.

- панель управления: https://panel.mittodatura.online
- документация openapi: https://panel.mittodatura.online/docs
- базовый url api: https://panel.mittodatura.online

## возможности

- получение актуального каталога и схем payload
- создание идемпотентных заказов через заголовок `Idempotency-Key`
- опрос статуса заказа до финального состояния (`COMPLETED`, `FAILED`, `REFUNDED`)
- проверка подписи входящих вебхуков (`HMAC-SHA256`)

## структура репозитория

- `python/mittodatura.py` — асинхронный клиент на `httpx` (python 3.10+)
- `node/mittodatura.mjs` — клиент без внешних зависимостей на встроенном `fetch` (node.js 18+)
- `go/mittodatura.go` — клиент на стандартной библиотеке (go 1.21+)
- `php/MittodaturaClient.php` — клиент на `curl` (php 8.1+)

## авторизация

все запросы к `/api/v1/*` требуют заголовок:

`X-Api-Key: b2b_live_...`

выпустить ключ и пополнить баланс можно в личном кабинете на `panel.mittodatura.online`. для тестирования без списания реальных средств используйте ключ окружения sandbox (`b2b_test_...`).

## быстрый старт (python)

установка зависимости:

```bash
pip install httpx
