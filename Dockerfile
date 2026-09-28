FROM public.ecr.aws/docker/library/golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /pix ./cmd/pix
FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
USER 10001:10001
COPY --from=build /pix /pix
EXPOSE 8080
ENTRYPOINT ["/pix"]
