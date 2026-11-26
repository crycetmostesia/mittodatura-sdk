import asyncio
import hashlib
import hmac
import time
from typing import Any

import httpx


class MittodaturaError(Exception):
    def __init__(self, code: str, message: str, status_code: int):
        super().__init__(f"{code}: {message} ({status_code})")
        self.code = code
        self.message = message
        self.status_code = status_code


class MittodaturaClient:
    def __init__(self, base_url: str, api_key: str, timeout: float = 15.0):
        self._client = httpx.AsyncClient(
            base_url=base_url.rstrip("/"),
            timeout=timeout,
            headers={
                "X-Api-Key": api_key,
                "Content-Type": "application/json",
            },
        )

    async def close(self) -> None:
        await self._client.aclose()

    async def __aenter__(self) -> "MittodaturaClient":
        return self

    async def __aexit__(self, *args: Any) -> None:
        await self.close()

    async def get_catalog(self) -> dict[str, Any]:
        resp = await self._client.get("/api/v1/catalog")
        return self._parse(resp)

    async def create_order(
        self,
        external_order_id: str,
        product_sku: str,
        customer_payload: dict[str, Any],
    ) -> dict[str, Any]:
        resp = await self._client.post(
            "/api/v1/orders",
            headers={"Idempotency-Key": str(external_order_id)},
            json={
                "productSku": product_sku,
                "customerPayload": customer_payload,
            },
        )
        return self._parse(resp)

    async def get_order(self, order_id: str) -> dict[str, Any]:
        resp = await self._client.get(f"/api/v1/orders/{order_id}")
        return self._parse(resp)

    async def fulfill_and_wait(
        self,
        external_order_id: str,
        product_sku: str,
        customer_payload: dict[str, Any],
        poll_interval: float = 1.0,
        timeout: float = 45.0,
    ) -> dict[str, Any]:
        created = await self.create_order(external_order_id, product_sku, customer_payload)
        order_id = created["orderId"]
        deadline = time.monotonic() + timeout

        while time.monotonic() < deadline:
            state = await self.get_order(order_id)
            if state.get("status") in ("COMPLETED", "FAILED", "REFUNDED"):
                return state
            await asyncio.sleep(poll_interval)

        raise TimeoutError(f"order {order_id} did not finish within {timeout}s")

    @staticmethod
    def verify_webhook(
        secret: str,
        raw_body: str,
        timestamp: str,
        signature: str,
        max_skew: int = 300,
    ) -> bool:
        try:
            ts = int(timestamp)
        except (TypeError, ValueError):
            return False

        if abs(int(time.time()) - ts) > max_skew:
            return False

        payload = f"{ts}.{raw_body}".encode("utf-8")
        expected = "sha256=" + hmac.new(secret.encode("utf-8"), payload, hashlib.sha256).hexdigest()
        return hmac.compare_digest(expected, signature.strip().lower())

    @staticmethod
    def _parse(resp: httpx.Response) -> dict[str, Any]:
        data = resp.json() if resp.content else {}
        if resp.is_success:
            return data
        err = data.get("error", {})
        raise MittodaturaError(
            code=err.get("code", f"HTTP_{resp.status_code}"),
            message=err.get("message", resp.text),
            status_code=resp.status_code,
        )
