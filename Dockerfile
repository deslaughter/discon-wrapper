# Use a newer golang image that supports Go 1.24
FROM golang:1.24 as build

# Set working directory
WORKDIR /build

# Copy the source code
COPY . .

# Set environment variables to bypass TLS certificate verification for Go modules
ENV GOPROXY="direct"
ENV GOSUMDB=off
ENV GOFLAGS="-insecure"

# Build the server
RUN go build -o /discon-server discon-wrapper/discon-server

# Create the final image
FROM ubuntu:24.04

# Install runtime dependencies including C, C++, and Fortran libraries
RUN apt-get update && apt-get install -y \
    libc6 \
    libstdc++6 \
    libgcc-s1 \
    libgfortran5 \
    liblapack3 \
    libblas3 \
    && rm -rf /var/lib/apt/lists/*

# Copy the binary from the build stage
COPY --from=build /discon-server /usr/local/bin/discon-server

# Create a directory for mounting the controller DLL
RUN mkdir -p /controller

# Set the working directory to where the controller will be mounted
WORKDIR /controller

# Expose the port the server listens on
EXPOSE 8080

# Start the server when the container runs
ENTRYPOINT ["/usr/local/bin/discon-server"]
CMD ["--port=8080"]

# Add labels
LABEL maintainer="Derek Slaughter"
LABEL description="DISCON-Wrapper server for bridging 64-bit OpenFAST with 32-bit controllers"
LABEL version="v0.1.0"