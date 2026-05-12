FROM golang:1.22-bookworm AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum* ./
COPY backend/ ./
RUN go mod tidy && go test ./... && CGO_ENABLED=0 go build -o /out/hostswitch ./cmd/hostswitch

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /out/hostswitch /app/hostswitch
EXPOSE 8080
ENTRYPOINT ["/app/hostswitch"]
