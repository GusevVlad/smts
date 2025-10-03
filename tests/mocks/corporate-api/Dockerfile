FROM golang:1.25-alpine

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -o corporate-api ./tests/mocks/corporate-api/cmd

EXPOSE 8080

CMD ["/app/corporate-api"]