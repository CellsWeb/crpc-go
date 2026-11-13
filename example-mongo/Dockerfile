FROM alpine:3.20

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY dist/server_linux /app/server
COPY .env /app/.env

RUN mkdir -p /app/public/uploads

EXPOSE 4001

CMD ["/app/server"]
