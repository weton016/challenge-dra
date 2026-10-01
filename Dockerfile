FROM public.ecr.aws/docker/library/golang:1.26-alpine AS build
RUN apk add --no-cache gcc musl-dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# The TigerBeetle client is a C library: link it statically to keep scratch.
RUN CGO_ENABLED=1 go build -trimpath -ldflags '-linkmode external -extldflags "-static"' -o /pix ./cmd/pix
FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
USER 10001:10001
COPY --from=build /pix /pix
EXPOSE 8080
ENTRYPOINT ["/pix"]
