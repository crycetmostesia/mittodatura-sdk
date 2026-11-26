import crypto from "node:crypto";

export class MittodaturaError extends Error {
  constructor(code, message, statusCode) {
    super(`${code}: ${message} (${statusCode})`);
    this.code = code;
    this.statusCode = statusCode;
  }
}

export class MittodaturaClient {
  constructor(baseUrl, apiKey, timeoutMs = 15000) {
    this.baseUrl = baseUrl.replace(/\/+$/, "");
    this.apiKey = apiKey;
    this.timeoutMs = timeoutMs;
  }

  async getCatalog() {
    return this.#request("GET", `${this.baseUrl}/api/v1/catalog`);
  }

  async createOrder({ externalOrderId, productSku, customerPayload }) {
    return this.#request(
      "POST",
      `${this.baseUrl}/api/v1/orders`,
      { productSku, customerPayload },
      { "Idempotency-Key": String(externalOrderId) }
    );
  }

  async getOrder(orderId) {
    return this.#request("GET", `${this.baseUrl}/api/v1/orders/${orderId}`);
  }

  async fulfillAndWait({
    externalOrderId,
    productSku,
    customerPayload,
    pollIntervalMs = 1000,
    timeoutMs = 45000,
  }) {
    const created = await this.createOrder({ externalOrderId, productSku, customerPayload });
    const deadline = Date.now() + timeoutMs;

    while (Date.now() < deadline) {
      const state = await this.getOrder(created.orderId);
      if (["COMPLETED", "FAILED", "REFUNDED"].includes(state.status)) {
        return state;
      }
      await new Promise((r) => setTimeout(r, pollIntervalMs));
    }

    throw new Error(`order ${created.orderId} did not finish within ${timeoutMs}ms`);
  }

  static verifyWebhook(secret, rawBody, timestamp, signature, maxSkewSec = 300) {
    const ts = Number.parseInt(timestamp, 10);
    if (!Number.isFinite(ts)) return false;
    if (Math.abs(Math.floor(Date.now() / 1000) - ts) > maxSkewSec) return false;

    const expected =
      "sha256=" +
      crypto.createHmac("sha256", secret).update(`${ts}.${rawBody}`).digest("hex");

    const a = Buffer.from(expected, "utf8");
    const b = Buffer.from(String(signature).trim().toLowerCase(), "utf8");
    return a.length === b.length && crypto.timingSafeEqual(a, b);
  }

  async #request(method, url, body = null, extraHeaders = {}) {
    const res = await fetch(url, {
      method,
      headers: {
        "X-Api-Key": this.apiKey,
        "Content-Type": "application/json",
        ...extraHeaders,
      },
      body: body ? JSON.stringify(body) : undefined,
      signal: AbortSignal.timeout(this.timeoutMs),
    });

    const text = await res.text();
    const data = text ? JSON.parse(text) : {};

    if (res.ok) return data;

    const err = data.error || {};
    throw new MittodaturaError(
      err.code || `HTTP_${res.status}`,
      err.message || text,
      res.status
    );
  }
}
