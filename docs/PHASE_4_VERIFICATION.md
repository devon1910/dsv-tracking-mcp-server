# Phase 4 Verification

Captured 2026-05-20. All three tools called end-to-end against the live DSV public tracking API via a headless Chromium browser (Cap.js solved automatically). Transport: MCP in-process (in-memory transport backed by the real browser stack).

Waybill `1806290829` resolves to shipment `LandStt:VAN5022058:CTTS:LAND` (SE→FR, 100% Delivered).

---

## 1. `list_reference_types`

**Request**
```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "list_reference_types",
    "arguments": {}
  }
}
```

**Response** (truncated after first 3 entries for brevity — full list is 21 entries)
```json
{
  "jsonrpc": "2.0",
  "result": {
    "content": [
      {
        "type": "text",
        "text": {
          "freshness": "static",
          "reference_types": [
            { "code": "Stt",       "label": "Shipment tracking number", "pattern": "^[A-Z]{2,8}\\d{5,12}$" },
            { "code": "WaybillNo", "label": "Waybill number",           "pattern": "^\\d{7,12}$" },
            { "code": "PackageId", "label": "Package ID",               "pattern": "^\\d{15,20}$" },
            "... 18 more entries ..."
          ]
        }
      }
    ],
    "isError": false
  }
}
```

---

## 2. `track_shipment` — waybill `1806290829`

**Request**
```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "track_shipment",
    "arguments": { "reference": "1806290829" }
  }
}
```

**Response**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "content": [
      {
        "type": "text",
        "text": {
          "freshness": "live",
          "retrieved_at": "2026-05-20T01:26:54Z",
          "shipments": [
            {
              "shipment_id":    "LandStt:VAN5022058:CTTS:LAND",
              "reference":      "VAN5022058",
              "status":         "Delivered",
              "transport_mode": "LAND",
              "data_provider":  "CTTS"
            }
          ]
        }
      }
    ],
    "isError": false
  }
}
```

---

## 3. `get_shipment_details` — shipment `LandStt:VAN5022058:CTTS:LAND`

**Request**
```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "get_shipment_details",
    "arguments": { "shipment_id": "LandStt:VAN5022058:CTTS:LAND" }
  }
}
```

**Response**
```json
{
  "jsonrpc": "2.0",
  "result": {
    "content": [
      {
        "type": "text",
        "text": {
          "freshness": "live",
          "retrieved_at": "2026-05-20T01:26:59Z",
          "shipment": {
            "shipment_id":    "LandStt:VAN5022058:CTTS:LAND",
            "reference":      "VAN5022058",
            "status":         "Delivered",
            "transport_mode": "LAND",
            "data_provider":  "CTTS",
            "sender": {
              "name":    null,
              "address": "46178 Sjuntorp",
              "city":    "Sjuntorp",
              "country": "Sweden"
            },
            "receiver": {
              "name":    null,
              "address": "62119 Dourges",
              "city":    "Dourges",
              "country": "France"
            },
            "events": [
              { "date": "2025-12-11T07:22:00Z", "code": "ENT", "raw_code": "ENT", "description": "Booked",          "location": "Vänersborg (SE)" },
              { "date": "2025-12-11T14:50:00Z", "code": "COL", "raw_code": "COL", "description": "Collected",       "location": "Sjuntorp (SE)" },
              { "date": "2025-12-11T22:00:00Z", "code": "ENM", "raw_code": "ENM", "description": "Arrived at hub",  "location": "Värnamo (SE)" },
              { "date": "2025-12-11T23:47:00Z", "code": "ENM", "raw_code": "ENM", "description": "Arrived at hub",  "location": "Värnamo (SE)" },
              { "date": "2025-12-15T23:24:00Z", "code": "MAN", "raw_code": "MAN", "description": "Departed",        "location": "Värnamo (SE)" },
              { "date": "2025-12-17T08:31:00Z", "code": "ENM", "raw_code": "ENM", "description": "Arrived at hub",  "location": "Serris (FR)" },
              { "date": "2025-12-17T21:00:00Z", "code": "MAN", "raw_code": "MAN", "description": "Departed",        "location": "Serris (FR)" },
              { "date": "2025-12-18T04:30:00Z", "code": "ENM", "raw_code": "ENM", "description": "Arrived at hub",  "location": "Saint-Omer (FR)" },
              { "date": "2025-12-18T07:20:00Z", "code": "DOT", "raw_code": "DOT", "description": "Out for delivery","location": "Saint-Omer (FR)" },
              { "date": "2025-12-18T10:11:00Z", "code": "DLV", "raw_code": "DLV", "description": "Delivered",       "location": "Dourges (FR)" }
            ],
            "view_in_ui_url": "https://mydsv.dsv.com/app/tracking-public/?refNumber=VAN5022058"
          }
        }
      }
    ],
    "isError": false
  }
}
```

---

## Tool registration check

Three tools confirmed registered:

```
tool: track_shipment
tool: get_shipment_details
tool: list_reference_types
```

Verified via in-process MCP client using the SDK's `NewInMemoryTransports()`.
The server also starts correctly over stdio: `make run` builds and launches the binary;
it prints `dsv-tracking-mcp-server ready` then blocks on stdin waiting for an MCP client.
