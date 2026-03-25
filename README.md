# pdf-extract

HTTP-сервис на **Go**: извлечение текста из PDF (без OCR), опционально вертикальная склейка страниц в один **PNG** с обрезкой белых полей. Источник PDF — **публичный URL** (JSON) или **загрузка файла** (`multipart/form-data`).

Рендер и текст выполняются через **Poppler** (`pdftotext`, `pdftoppm`) внутри контейнера.

## Быстрый старт (локально)

Требования: Go 1.22+, в `PATH` установлены `poppler` (macOS: `brew install poppler`).

```bash
export PUBLIC_BASE_URL=http://localhost:8000
go run ./cmd/pdf-extract
```

## Переменные окружения

| Переменная | Обязательно | По умолчанию | Описание |
|------------|-------------|--------------|----------|
| `PUBLIC_BASE_URL` | да | — | Публичный origin для абсолютных ссылок на PNG (без завершающего `/`) |
| `PORT` | нет | `8000` | Порт HTTP |
| `UPLOAD_DIR` | нет | `uploads` | Каталог временных PDF |
| `OUTPUT_DIR` | нет | `outputs` | Каталог временных PNG |
| `MAX_UPLOAD_BYTES` | нет | `33554432` | Лимит тела multipart (32 MiB) |
| `MAX_DOWNLOAD_BYTES` | нет | `33554432` | Лимит скачивания PDF по URL |
| `HTTP_FETCH_TIMEOUT` | нет | `120s` | Таймаут исходящего HTTP при загрузке по URL |
| `FILE_TTL` | нет | `1h` | Через сколько удалить загруженный PDF и PNG |
| `RENDER_DPI` | нет | `150` | DPI для `pdftoppm` |

## API

### POST `/v1/process`

Ровно один режим тела запроса.

#### 1) JSON — PDF по URL

`Content-Type: application/json`

```json
{
  "source": { "type": "url", "url": "https://example.com/doc.pdf" },
  "options": {
    "render_image": false,
    "crop_margins": true
  }
}
```

- **`options`** опционален.
- **`render_image`**: по умолчанию `false` (только текст, меньше нагрузка). Если `true` — генерируется PNG.
- **`crop_margins`**: по умолчанию `true`; учитывается только при `render_image: true`.

Ответ `200`:

```json
{
  "status": "success",
  "text": "…",
  "image": null
}
```

При `render_image: true`:

```json
{
  "status": "success",
  "text": "…",
  "image": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "url": "https://your-host/v1/files/550e8400-e29b-41d4-a716-446655440000"
  }
}
```

Страницы в `text` разделяются двойным переводом строки (`\n\n`), как в прежнем Python-сервисе.

#### 2) Multipart — файл

`Content-Type: multipart/form-data`

- Поле **`file`**: PDF (обязательно).
- Поле **`options`**: JSON-строка с теми же ключами, что выше (опционально).

Пример:

```bash
curl -sS -X POST "$BASE/v1/process" \
  -F "file=@./document.pdf" \
  -F 'options={"render_image":false}' \
  --max-time 120
```

### GET `/v1/files/{id}`

Отдаёт PNG по `id` из ответа `image.id`. Файлы удаляются после `FILE_TTL`.

### GET `/health` и GET `/v1/health`

Проверка живости и счётчики файлов в каталогах (как ориентир).

## Ошибки

Ответы с телом **`application/problem+json`** (RFC 7807), поля: `type`, `title`, `detail`, `status`.

## Docker

Скопируйте `.env.example` в `.env` и задайте `PUBLIC_BASE_URL`.

```bash
docker compose up --build -d
```

Образ включает `poppler-utils`.

### Traefik (ops3)

В репозитории инфраструктуры VDS:

- `servers/3 - apps/services/traefik/config/sablier-apps.yml` — хост **`pdf-extract.l1979.ru`**, пути `/health`, `/v1/health`, префикс `/v1` (Sablier + бэкенд `http://pdf-extract:8000`).
- `servers/3 - apps/services/traefik/config/middlewares.yml` — **`pdf-extract-buffering`**: до **32 MiB** тела запроса (как `MAX_UPLOAD_BYTES` по умолчанию).

Нужна DNS-запись **`pdf-extract.l1979.ru`** на IP сервера с Traefik. В контейнере задайте **`PUBLIC_BASE_URL=https://pdf-extract.l1979.ru`**. После правок динамических файлов перезагрузите Traefik или дождитесь подхвата файлов провайдером.

## Безопасность загрузки по URL

Запрещены хосты `localhost`, частные и link-local адреса по результатам DNS; разрешены только схемы `http`/`https`. При необходимости жёстче ограничьте исходящий доступ на уровне сети.

## Отличия от legacy pdf2image (Python / PyMuPDF)

Текст и растровый вывод могут **слегка отличаться** от PyMuPDF/Poppler — перед cutover сравните на своих эталонных PDF.

## Лицензия

Внутренний проект; при публикации уточните лицензию.
