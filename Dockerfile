FROM golang:1.26 as builder

WORKDIR /workspace
COPY go.* .
RUN go mod download
COPY main.go main.go
COPY internal internal
RUN go test ./...
RUN CGO_ENABLED=0 GOOS=linux go build -a -o /go-rotate-backups main.go


# Use distroless as minimal base image to package the project
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=builder /go-rotate-backups .
ENTRYPOINT ["/go-rotate-backups"]
