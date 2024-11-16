# Word of Wisdom: Proof of Work Implementation

This repository contains a TCP-based "Word of Wisdom" server and client implemented in Go. The server uses a Proof of Work (PoW) mechanism to mitigate DDoS attacks. Once the client successfully solves the PoW challenge, the server sends a random quote.

## Features

- **Proof of Work Protection**: Implements a challenge-response protocol based on a configurable difficulty level.
- **Structured Logging**: Uses the `zap` logger for production-grade structured logging.
- **Configuration**: Uses a `config.yaml` file for runtime configuration of both the server and client.
- **Dockerized**: Includes `Dockerfile` and `docker-compose.yml` for containerized deployment.
- **Compatibility**: Can run with Docker or Podman.

---

## Architecture

### Server Workflow
1. Starts a TCP server on a configurable address and port.
2. For each connection:
   - Generates a challenge (random string), difficulty level, and timestamp.
   - Sends the challenge to the client.
   - Verifies the client's solution based on the challenge, nonce, and timestamp.
   - Sends a random quote if the PoW is valid; otherwise, rejects the solution.

### Client Workflow
1. Connects to the server.
2. Receives the PoW challenge, difficulty, and timestamp.
3. Computes a valid nonce by brute-forcing to meet the required difficulty.
4. Sends the solution (nonce and timestamp) back to the server.
5. Receives a quote if the PoW is valid.

---

## Requirements

- Go 1.23+
- Docker (or Podman)
- `docker-compose` (or `podman-compose`)

---

## Configuration

The `config.yaml` file defines runtime settings for the server and client:

```yaml
server:
  host: "0.0.0.0"
  port: 9999
  max_connections: 100
  connection_timeout: 10s
  time_window: 5m
  min_difficulty: 4
  max_difficulty: 6

client:
  server_address: "localhost:9999"
  connection_timeout: 10s
  max_nonce: 100000000
```

## Running the Solution

### With Docker Compose

1. **Build and Run**
    ```bash
    docker-compose up --build
    ```
    
    Scale Clients (Optional) To simulate multiple clients:
    ```bash
    docker-compose up --scale client=3 --build
    ```

2. **Stop Services**
```bash
docker-compose down
```

## With Podman Compose
1. **Build and Run**
```bash
podman-compose up --build
```
2. **Stop Services** 
```bash
podman-compose down
```
