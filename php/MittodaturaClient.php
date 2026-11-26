<?php

declare(strict_types=1);

final class MittodaturaException extends RuntimeException
{
    public function __construct(
        public readonly string $errorCode,
        string $message,
        public readonly int $statusCode
    ) {
        parent::__construct("{$errorCode}: {$message} ({$statusCode})", $statusCode);
    }
}

final class MittodaturaClient
{
    private string $baseUrl;

    public function __construct(
        string $baseUrl,
        private readonly string $apiKey,
        private readonly int $timeoutSec = 15
    ) {
        $this->baseUrl = rtrim($baseUrl, '/');
    }

    public function getCatalog(): array
    {
        return $this->request('GET', $this->baseUrl . '/api/v1/catalog');
    }

    public function createOrder(string $externalOrderId, string $productSku, array $customerPayload): array
    {
        return $this->request(
            'POST',
            $this->baseUrl . '/api/v1/orders',
            [
                'productSku' => $productSku,
                'customerPayload' => (object)$customerPayload,
            ],
            ['Idempotency-Key: ' . $externalOrderId]
        );
    }

    public function getOrder(string $orderId): array
    {
        return $this->request('GET', $this->baseUrl . '/api/v1/orders/' . rawurlencode($orderId));
    }

    public function fulfillAndWait(
        string $externalOrderId,
        string $productSku,
        array $customerPayload,
        int $pollIntervalMs = 1000,
        int $timeoutSec = 45
    ): array {
        $created = $this->createOrder($externalOrderId, $productSku, $customerPayload);
        $orderId = $created['orderId'];
        $deadline = microtime(true) + $timeoutSec;

        while (microtime(true) < $deadline) {
            $state = $this->getOrder($orderId);
            if (in_array($state['status'] ?? '', ['COMPLETED', 'FAILED', 'REFUNDED'], true)) {
                return $state;
            }
            usleep($pollIntervalMs * 1000);
        }

        throw new RuntimeException("order {$orderId} did not finish within {$timeoutSec}s");
    }

    public static function verifyWebhook(
        string $secret,
        string $rawBody,
        string $timestamp,
        string $signature,
        int $maxSkewSec = 300
    ): bool {
        if (!ctype_digit($timestamp)) {
            return false;
        }
        if (abs(time() - (int)$timestamp) > $maxSkewSec) {
            return false;
        }

        $expected = 'sha256=' . hash_hmac('sha256', "{$timestamp}.{$rawBody}", $secret);
        return hash_equals($expected, strtolower(trim($signature)));
    }

    private function request(string $method, string $url, ?array $body = null, array $extraHeaders = []): array
    {
        $ch = curl_init($url);
        $headers = array_merge([
            'X-Api-Key: ' . $this->apiKey,
            'Content-Type: application/json',
        ], $extraHeaders);

        curl_setopt_array($ch, [
            CURLOPT_CUSTOMREQUEST => $method,
            CURLOPT_HTTPHEADER => $headers,
            CURLOPT_RETURNTRANSFER => true,
            CURLOPT_TIMEOUT => $this->timeoutSec,
        ]);

        if ($body !== null) {
            curl_setopt($ch, CURLOPT_POSTFIELDS, json_encode($body, JSON_UNESCAPED_UNICODE));
        }

        $raw = curl_exec($ch);
        $status = (int)curl_getinfo($ch, CURLINFO_HTTP_CODE);
        $curlErr = curl_error($ch);
        curl_close($ch);

        if ($raw === false) {
            throw new RuntimeException("curl error: {$curlErr}");
        }

        $data = $raw !== '' ? (json_decode($raw, true) ?? []) : [];
        if ($status >= 200 && $status < 300) {
            return $data;
        }

        $err = $data['error'] ?? [];
        throw new MittodaturaException(
            $err['code'] ?? "HTTP_{$status}",
            $err['message'] ?? $raw,
            $status
        );
    }
}
