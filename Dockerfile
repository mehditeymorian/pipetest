# syntax=docker/dockerfile:1

FROM golang:1.24 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/pipetest ./cmd/pipetest

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/pipetest /usr/local/bin/pipetest
ENTRYPOINT ["/usr/local/bin/pipetest"]
CMD ["--help"]
