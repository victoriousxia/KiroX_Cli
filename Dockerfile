# Stage 1: Build frontend
FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.24-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download || true
COPY . .
COPY --from=frontend /app/web/dist ./server/dist
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o kirox-server ./cmd/server

# Stage 3: Final image
FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=backend /app/kirox-server .
RUN mkdir -p /app/data
EXPOSE 8080
ENV DATA_DIR=/app/data
ENV PORT=8080
ENV ADMIN_PASSWORD=admin
ENTRYPOINT ["./kirox-server"]
