# Contexto de build = raiz do lab (labs/instrumentation).
# SRC escolhe de onde vem o main.go: app-go (enunciado) ou solutions/app-go (gabarito).
FROM golang:1.27-alpine AS build
ARG SRC=app-go
WORKDIR /src
COPY app-go/go.mod app-go/go.sum ./
RUN go mod download
COPY ${SRC}/main.go ./
RUN CGO_ENABLED=0 go build -o /app .

FROM alpine:3.23
COPY --from=build /app /app
USER 65534
EXPOSE 8080
ENTRYPOINT ["/app"]
