# Build context = raiz do repositório.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY generator/go.mod generator/go.sum ./
RUN go mod download
COPY generator/*.go ./
# Cada lição traz seu próprio cenário em functions/<fn>/setup/scenario.go.
# tools/deploy.sh copia os cenários (já checados) para .build/scenarios/
# como zz_<fn>_scenario.go — todos no mesmo package main.
COPY .build/scenarios/ ./
RUN CGO_ENABLED=0 go build -o /generator .

FROM gcr.io/distroless/static:nonroot
COPY --from=build /generator /generator
EXPOSE 8080
ENTRYPOINT ["/generator"]
