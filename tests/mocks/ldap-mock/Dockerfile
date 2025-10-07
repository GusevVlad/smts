FROM golang:1.25-alpine

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -o ldap-mock ./tests/mocks/ldap-mock/cmd

EXPOSE 1389

CMD ["/app/ldap-mock"]