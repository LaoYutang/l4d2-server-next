# Stage 1: Build Frontend
FROM node:24-alpine AS web-builder
WORKDIR /app/frontend
COPY ./frontend .
RUN npm install
RUN npm run build

# Stage 2: Build Backend
FROM golang:1.25-alpine AS builder
WORKDIR /app/backend
COPY ./backend .
RUN go build -o /app/l4d2-manager

# Stage 3: Final Image
FROM docker:29.1.1-cli-alpine3.22
EXPOSE 27020
COPY --from=builder /app/l4d2-manager /
COPY --from=web-builder /app/backend/static /static
ENTRYPOINT ["/l4d2-manager"]
