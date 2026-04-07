# Relay Webhook 接入指南

本文档描述 KodaClaw Community 的 Relay webhook 严格接入方式。

## 入口

- URL: `/api/v1/webhook/incoming/:instanceId`
- Method: `POST`
- Content-Type: `application/json`

其中 `instanceId` 是 Relay 实例 ID，用于路由到目标 KodaClaw Relay account。

## 安全头

请求必须携带以下签名头：

- `X-Relay-Timestamp`: Unix 秒级时间戳
- `X-Relay-Signature`: `hex(HMAC_SHA256(secret, timestamp + "." + rawBody))`
- `X-Relay-KeyId`: 可选；多密钥模式下建议显式携带，可填写 key ID 或 key name

校验失败时，请求会在 ingress 层直接拒绝，不进入 Relay 转发链路。

## 严格请求体

Relay Community 只接受以下严格格式，不兼容旧字段别名，不自动补默认值，也不会从 `payload` 猜顶层协议字段。

```json
{
  "schemaVersion": "1.0",
  "eventType": "MessageReceived",
  "threadType": "DirectMessage",
  "externalThreadId": "crypto-monitor:BTC:4h",
  "externalMessageId": "btc-4h-2026-04-07T02:00:00Z",
  "text": "BTC 4h 下跌 1.5%，请分析是否需要动作。",
  "sender": {
    "id": "crypto-monitor",
    "displayName": "Crypto Monitor",
    "isBot": true
  },
  "occurredAt": "2026-04-07T02:00:00Z",
  "correlationId": "monitor-run-123",
  "payload": {
    "symbol": "BTC",
    "changePct": -1.5
  }
}
```

## 字段要求

- `schemaVersion`: 目前只接受 `"1.0"`
- `eventType`: 只接受 `MessageReceived` 或 `NotificationReceived`
- `threadType`: 只接受 `DirectMessage` 或 `Group`
- `externalThreadId`: 必须是稳定线程 ID，不能为空
- `externalMessageId`: 必须是业务消息幂等键，不能为空
- `text`: `MessageReceived` 必须提供非空字符串；`NotificationReceived` 推荐提供摘要
- `sender.id`: 必填，非空
- `sender.displayName`: 必填，非空
- `sender.isBot`: 必填，布尔值
- `occurredAt`: 必填，必须是 RFC3339 时间字符串
- `payload`: 可选；如果存在，必须是 JSON object

## 不被接受的输入

以下请求会直接被拒绝：

- 只有 `payload`，但缺失顶层 `text`
- 使用 `senderId` / `senderName` 等旧别名
- 使用 `threadId` / `chatId` / `conversationId` 代替 `externalThreadId`
- 使用 `timestamp` 代替 `occurredAt`
- `payload` 不是 object
- 带未知字段的 body

## curl 示例

```bash
timestamp=$(date +%s)
body='{"schemaVersion":"1.0","eventType":"MessageReceived","threadType":"DirectMessage","externalThreadId":"demo-thread","externalMessageId":"demo-message-'$timestamp'","text":"hello from strict webhook","sender":{"id":"demo-bot","displayName":"Demo Bot","isBot":true},"occurredAt":"2026-04-07T02:00:00Z","payload":{"source":"curl-example"}}'
sig=$(printf '%s' "$timestamp.$body" | openssl dgst -sha256 -hmac "YOUR_WEBHOOK_KEY" | awk '{print $2}')

curl -X POST "https://community.ai-koda.com/api/v1/webhook/incoming/YOUR_INSTANCE_ID" \
  -H "Content-Type: application/json" \
  -H "X-Relay-Timestamp: $timestamp" \
  -H "X-Relay-Signature: $sig" \
  -d "$body"
```

如果你使用多把 webhook key，可以额外带上：

```bash
-H "X-Relay-KeyId: YOUR_KEY_ID"
```

## 响应

成功：

```json
{
  "accepted": true,
  "eventId": "relay_evt_xxx"
}
```

失败：

```json
{
  "accepted": false,
  "errorCode": "missing_required_field",
  "message": "Missing required field: externalThreadId"
}
```

常见 `errorCode`：

- `invalid_instance`
- `signature_invalid`
- `timestamp_expired`
- `payload_required`
- `payload_not_json`
- `payload_not_object`
- `unsupported_schema_version`
- `unsupported_event_type`
- `unsupported_thread_type`
- `missing_required_field`
- `invalid_field_type`
- `invalid_timestamp`
- `payload_too_large`
