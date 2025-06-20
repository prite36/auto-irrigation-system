# Stage 1: Dependencies
# Use the official Golang image to create a build artifact.
# This stage is for downloading Go modules.
FROM golang:1.24-alpine AS deps

# Set the working directory inside the container.
WORKDIR /app

# Copy go.mod and go.sum files to the workspace.
COPY go.mod go.sum ./

# Download all dependencies.
# This will be cached if go.mod and go.sum don't change.
RUN go mod download

# Stage 2: Builder
# This stage builds the application.
FROM deps AS builder

# Copy the entire source code.
COPY . .

# Build the Go app.
# CGO_ENABLED=0 builds a static binary.
# -o /app/main creates the binary named 'main' in the /app directory.
# The entry point is cmd/irrigation/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -mod=readonly -o /app/main ./cmd/irrigation/main.go

# Stage 3: Production
# Start from a minimal base image for a small footprint.
FROM alpine:latest AS production

# Install timezone data and set the timezone
RUN apk add --no-cache tzdata
ENV TZ Asia/Bangkok

# Set the working directory.
WORKDIR /app

# Copy the static binary from the builder stage.
COPY --from=builder /app/main .

# Copy the devices configuration file.
COPY devices.json .

# Expose port 8080 for the application.
# You can change this to match your application's port.
EXPOSE 8080

# Command to run the executable.
CMD ["./main"]
