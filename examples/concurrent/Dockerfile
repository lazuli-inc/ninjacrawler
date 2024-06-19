# Use Ubuntu 22.04 alpha as a base image
FROM ubuntu:22.04

# Install Go
RUN apt-get update && \
    apt-get install -y wget && \
    wget https://golang.org/dl/go1.22.3.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.22.3.linux-amd64.tar.gz && \
    rm go1.22.3.linux-amd64.tar.gz

# Set Go environment variables
ENV PATH="/usr/local/go/bin:${PATH}"

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum first to leverage Docker layer caching
COPY go.mod go.sum ./

# Download Go dependencies
RUN go mod download

# Copy the rest of the application code
COPY . .

# Install Playwright CLI with the right version for later use
RUN PWGO_VER=$(grep -oE "playwright-go v\S+" /app/go.mod | sed 's/playwright-go //g') \
    && go install github.com/playwright-community/playwright-go/cmd/playwright@${PWGO_VER}

# Install dependencies and all browsers (or specify one)
RUN go run github.com/playwright-community/playwright-go/cmd/playwright@latest install --with-deps



# Build and run the Go application
CMD ["go", "run", "main.go"]
