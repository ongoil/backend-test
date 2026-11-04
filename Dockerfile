# ---- Build stage ----
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server .

# ---- Runtime stage ----
# distroless/static has no shell or package manager and runs as non-root.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server

EXPOSE 8080 50051
USER nonroot:nonroot
ENTRYPOINT ["/server"]
