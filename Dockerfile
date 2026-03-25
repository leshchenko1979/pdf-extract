# Build static binary (Poppler is invoked via CLI, not CGO).
FROM golang:1.22-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/pdf-extract ./cmd/pdf-extract

FROM alpine:3.20
RUN apk add --no-cache poppler-utils ca-certificates curl \
	&& rm -rf /var/cache/apk/*
WORKDIR /app
RUN mkdir -p uploads outputs \
	&& chown -R nobody:nobody /app
COPY --from=build /out/pdf-extract /usr/local/bin/pdf-extract
ENV PORT=8000
EXPOSE 8000
USER nobody
CMD ["pdf-extract"]
