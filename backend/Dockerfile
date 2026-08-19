FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/undeschimb-api ./cmd/api

FROM alpine:3.21
RUN adduser -D -H appuser
WORKDIR /app
COPY --from=build /out/undeschimb-api ./undeschimb-api
COPY migrations ./migrations
USER appuser
EXPOSE 8080
CMD ["./undeschimb-api"]
