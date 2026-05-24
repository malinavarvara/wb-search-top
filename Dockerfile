# Этап 1: Сборка
FROM golang:1.25 AS build

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o searcher_top_bin ./searcher_top/main.go

FROM alpine:3.20

WORKDIR /root/

COPY --from=build /app/searcher_top_bin .

COPY --from=build /app/searcher_top/config.yaml ./searcher_top/config.yaml

EXPOSE 8080

CMD ["./searcher_top_bin", "-config=searcher_top/config.yaml"]