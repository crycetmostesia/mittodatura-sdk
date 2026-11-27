# mittodatura sdk

клиенты и примеры интеграции для b2b api шлюза цифровых товаров mittodatura.

- панель управления: https://panel.mittodatura.online
- документация openapi: https://panel.mittodatura.online/docs
- базовый url api: https://panel.mittodatura.online
- сайт и условия: https://mittodatura.online
- API пополнения Steam: https://mittodatura.online/steam-topup-api

## возможности

- получение актуального каталога (`tree` и `items`) со схемами `payloadExample`
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

`X-Api-Key: mittodatura_live_...`

выпустить ключ и пополнить баланс можно в кабинете на `panel.mittodatura.online`. для тестов без списания средств используется ключ окружения sandbox (`mittodatura_test_...`).

## быстрый старт (python)

установка зависимости:

```bash
pip install httpx
```

пример использования в коде магазина:

```python
import asyncio
from mittodatura import MittodaturaClient

async def main():
    async with MittodaturaClient("https://panel.mittodatura.online", "mittodatura_live_your_key") as client:
        catalog = await client.get_catalog()
        order = await client.fulfill_and_wait(
            external_order_id="shop_order_10492",
            product_sku="STEAM_TOPUP_1USD",
            customer_payload={"login": "steam_user_login"},
        )
        print(order)

asyncio.run(main())
```

## быстрый старт (node.js)

внешние пакеты не требуются:

```javascript
import { MittodaturaClient } from "./node/mittodatura.mjs";

const client = new MittodaturaClient("https://panel.mittodatura.online", "mittodatura_live_your_key");

const order = await client.fulfillAndWait({
  externalOrderId: "shop_order_10492",
  productSku: "STEAM_TOPUP_1USD",
  customerPayload: { login: "steam_user_login" },
});

console.log(order);
```

## эндпоинты

- `GET /api/v1/catalog` — возвращает `updatedAtUtc`, сгруппированное дерево `tree` и плоский список `items` с полями `payloadExample` и `requiredPayloadFields`
- `POST /api/v1/orders` — создание заказа; обязателен заголовок `Idempotency-Key` (уникальный id заказа в вашей системе)
- `GET /api/v1/orders/{orderId}` — получение статуса заказа и данных выдачи

## проверка вебхуков

при указании webhook url в кабинете шлюз отправляет post-запросы с заголовками:

- `X-Timestamp` — unix-время отправки в секундах
- `X-Signature` — строка вида `sha256=<hex>`

подпись вычисляется как `hmac_sha256(secret, "{timestamp}.{raw_body}")`. метод `verify_webhook` (и его аналоги в других языках) проверяет подпись и отклоняет запросы старше 300 секунд.

## обработка ошибок

при статусе ответа `>= 400` api возвращает json:

```json
{
  "error": {
    "code": "INSUFFICIENT_FUNDS",
    "message": "not enough balance"
  }
}
```
