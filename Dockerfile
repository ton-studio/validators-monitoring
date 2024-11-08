FROM golang:1.23 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o validator-health .

FROM node:22.9 AS frontend-builder

WORKDIR /app/frontend

COPY frontend/package*.json ./
RUN npm install

COPY frontend/ ./
RUN npm run build


FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/validator-health .
COPY --from=frontend-builder /app/frontend/build ./static

RUN apk --no-cache add ca-certificates

EXPOSE 3000

CMD ["./validator-health"]
