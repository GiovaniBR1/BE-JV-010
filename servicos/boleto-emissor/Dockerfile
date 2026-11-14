FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget
COPY --from=build /out/api /api
# O healthcheck usa wget como nobody. /tmp é gravável; a home padrão não.
ENV HOME=/tmp
USER nobody
EXPOSE 8080
ENTRYPOINT ["/api"]
