FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -o /app .

FROM gcr.io/distroless/static:nonroot
COPY --from=build /app /app
EXPOSE 8080
HEALTHCHECK --interval=2s --timeout=2s --retries=15 CMD ["/app", "healthcheck"]
ENTRYPOINT ["/app"]
