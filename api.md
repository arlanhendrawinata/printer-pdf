# Printer REST API

REST API untuk print PDF menggunakan Go Fiber.

## Setup

1. **Install dependencies**
```bash
go get github.com/gofiber/fiber/v2
go get github.com/gofiber/fiber/v2/middleware/cors
go get github.com/gofiber/fiber/v2/middleware/logger
```

2. **Run server**
```bash
go run api.go
```

Server akan jalan di `http://localhost:3000`

## API Endpoints

### 1. Print PDF
**POST** `/print`

Print file PDF yang ada di project folder.

**Request Body:**
```json
{
  "file_name": "test.pdf",
  "printer": "MP230",
  "settings": {
    "paper_size": "a4",
    "color": "color",
    "double_sided": false,
    "duplex_mode": "vertical",
    "copies": 1
  }
}
```

**Response (Success):**
```json
{
  "success": true,
  "message": "Print job sent successfully",
  "job_id": "job_1738740123"
}
```

**Response (Error):**
```json
{
  "success": false,
  "error": "File not found: test.pdf"
}
```

**Example cURL:**
```bash
curl -X POST http://localhost:3000/print \
  -H "Content-Type: application/json" \
  -d '{
    "file_name": "test.pdf",
    "printer": "MP230",
    "settings": {
      "paper_size": "a4",
      "color": "color",
      "copies": 1
    }
  }'
```

**Example JavaScript (fetch):**
```javascript
fetch('http://localhost:3000/print', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({
    file_name: 'test.pdf',
    printer: 'MP230',
    settings: {
      paper_size: 'a4',
      color: 'color',
      double_sided: false,
      copies: 1
    }
  })
})
.then(res => res.json())
.then(data => console.log(data));
```

### 2. Get Printer Status
**GET** `/printer/status/:name`

Cek status printer.

**Example:**
```bash
curl http://localhost:3000/printer/status/MP230
```

**Response:**
```json
{
  "success": true,
  "data": {
    "name": "MP230",
    "status": "Ready",
    "jobs_in_queue": 0,
    "is_ready": true,
    "has_paper": true,
    "has_error": false
  }
}
```

### 3. List Available Files
**GET** `/files`

List semua file PDF di project folder.

**Example:**
```bash
curl http://localhost:3000/files
```

**Response:**
```json
{
  "success": true,
  "count": 2,
  "files": [
    {
      "name": "test.pdf",
      "size": 52481,
      "modified": "2024-02-05T10:30:00Z"
    },
    {
      "name": "document.pdf",
      "size": 125600,
      "modified": "2024-02-05T11:15:00Z"
    }
  ]
}
```

## Settings Options

### paper_size
- `a4` (default)
- `letter`
- `legal`
- `a5`

### color
- `color` (default) - Full color
- `monochrome` - Hitam putih

### double_sided
- `false` (default) - Print satu sisi
- `true` - Print bolak-balik

### duplex_mode
- `vertical` (default) - Long edge flip
- `horizontal` - Short edge flip

### copies
- Integer (default: 1)

## Error Codes

- `400` - Bad request (invalid JSON)
- `404` - File atau printer tidak ditemukan
- `500` - Internal server error (Ghostscript error, print failure)
- `503` - Printer not ready

## Testing dengan Postman

1. Import collection atau buat request baru
2. Set method POST ke `http://localhost:3000/print`
3. Set header `Content-Type: application/json`
4. Paste JSON di body (raw)
5. Send!

## Production Tips

- Set environment variable untuk port: `PORT=8080`
- Tambah authentication middleware
- Add rate limiting
- Log print jobs ke database
- Implement job queue untuk concurrent requests