# ---- Build stage ----
FROM golang:1.25-alpine AS build

RUN apk add --no-cache git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/api    ./cmd/api \
 && CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/worker ./cmd/worker \
 && CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/toi    ./cmd/toi

# ---- Runtime stage ----
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

COPY --from=build /bin/api    /usr/local/bin/api
COPY --from=build /bin/worker /usr/local/bin/worker
COPY --from=build /bin/toi    /usr/local/bin/toi
COPY migrations/ /app/migrations/

WORKDIR /app

EXPOSE 8080

ENTRYPOINT ["api"]
