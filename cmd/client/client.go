package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/dmitriykara/word-of-wisdom-pow/internal/config"
	"go.uber.org/zap"
)

// WordOfWisdomClient is a client that connects to the server and solves PoW challenges
type WordOfWisdomClient struct {
	config config.ClientConfig
	logger *zap.Logger
}

// NewClient initializes a new client with the given configuration and logger
func NewClient(cfg config.ClientConfig, logger *zap.Logger) *WordOfWisdomClient {
	return &WordOfWisdomClient{
		config: cfg,
		logger: logger,
	}
}

// Run starts the client, solves the PoW challenge, and interacts with the server
func (c *WordOfWisdomClient) Run(ctx context.Context) error {
	conn, err := net.Dial("tcp", c.config.ServerAddress)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	defer func() {
		_ = conn.Close()
	}()

	c.logger.Info("Connected to server", zap.String("address", c.config.ServerAddress))

	// Set connection timeout
	err = conn.SetDeadline(time.Now().Add(c.config.ConnectionTimeout))
	if err != nil {
		c.logger.Warn("set deadline failed", zap.Error(err))
	}

	// Receive challenge from server
	challenge, serverTimestamp, difficulty, err := c.receiveChallenge(conn)
	if err != nil {
		c.logger.Error("Failed to receive challenge", zap.Error(err))

		return err
	}

	c.logger.Info("Challenge received",
		zap.String("challenge", challenge),
		zap.Time("serverTimestamp", serverTimestamp),
		zap.Int("difficulty", difficulty),
	)

	// Solve PoW challenge
	nonce, err := c.solvePoW(ctx, challenge, serverTimestamp, difficulty)
	if err != nil {
		c.logger.Error("Failed to solve PoW", zap.Error(err))

		return err
	}

	c.logger.Info("PoW solved", zap.String("nonce", nonce))

	// Send solution to server
	clientTimestamp := time.Now().UTC()
	if err := c.sendResponse(conn, nonce, clientTimestamp); err != nil {
		c.logger.Error("Failed to send response", zap.Error(err))

		return err
	}

	// Receive server response (quote or error)
	if err := c.receiveServerResponse(conn); err != nil {
		c.logger.Error("Failed to receive server response", zap.Error(err))

		return err
	}

	return nil
}

// receiveChallenge reads the challenge message from the server
func (c *WordOfWisdomClient) receiveChallenge(conn net.Conn) (string, time.Time, int, error) {
	buffer := make([]byte, 4096)
	n, err := conn.Read(buffer)
	if err != nil {
		return "", time.Time{}, 0, err
	}

	message := strings.TrimSpace(string(buffer[:n]))
	parts := strings.Split(message, ";")
	if len(parts) != 3 {
		return "", time.Time{}, 0, fmt.Errorf("invalid challenge format")
	}

	challenge := strings.TrimPrefix(parts[0], "Challenge:")
	timestampStr := strings.TrimPrefix(parts[1], "Timestamp:")
	difficultyStr := strings.TrimPrefix(parts[2], "Difficulty:")

	serverTimestamp, err := time.Parse(time.RFC3339Nano, timestampStr)
	if err != nil {
		return "", time.Time{}, 0, fmt.Errorf("invalid timestamp format: %w", err)
	}

	difficulty, err := strconv.Atoi(difficultyStr)
	if err != nil {
		return "", time.Time{}, 0, fmt.Errorf("invalid difficulty value: %w", err)
	}

	return challenge, serverTimestamp, difficulty, nil
}

// solvePoW solves the Proof of Work challenge
func (c *WordOfWisdomClient) solvePoW(ctx context.Context, challenge string, serverTimestamp time.Time, difficulty int) (string, error) {
	var nonce int
	requiredPrefix := strings.Repeat("0", difficulty)

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
			data := fmt.Sprintf("%s%d%s", challenge, nonce, serverTimestamp.Format(time.RFC3339Nano))

			hash := sha256.Sum256([]byte(data))
			hashHex := hex.EncodeToString(hash[:])

			if strings.HasPrefix(hashHex, requiredPrefix) {
				return strconv.Itoa(nonce), nil
			}

			nonce++
		}
	}
}

// sendResponse transmits the nonce and client timestamp to the server
func (c *WordOfWisdomClient) sendResponse(conn net.Conn, nonce string, timestamp time.Time) error {
	message := fmt.Sprintf("Nonce:%s;Timestamp:%s\n", nonce, timestamp.Format(time.RFC3339Nano))
	_, err := conn.Write([]byte(message))

	return err
}

// receiveServerResponse reads the server's response (quote or error)
func (c *WordOfWisdomClient) receiveServerResponse(conn net.Conn) error {
	buffer := make([]byte, 4096)
	n, err := conn.Read(buffer)
	if err != nil {
		return err
	}

	response := strings.TrimSpace(string(buffer[:n]))
	if strings.HasPrefix(response, "Quote:") {
		quote := strings.TrimPrefix(response, "Quote:")
		c.logger.Info("Received quote", zap.String("quote", quote))
	} else if strings.HasPrefix(response, "Error:") {
		errorMessage := strings.TrimPrefix(response, "Error:")
		c.logger.Warn("Received error from server", zap.String("error", errorMessage))
	} else {
		c.logger.Warn("Unknown server response", zap.String("response", response))
	}

	return nil
}

// main entry point of the client application
func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to initialize zap logger: %v", err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
	}

	client := NewClient(cfg.Client, logger)
	ctx, cancel := context.WithTimeout(context.Background(), client.config.ConnectionTimeout)
	defer cancel()

	if err := client.Run(ctx); err != nil {
		logger.Fatal("Client encountered an error", zap.Error(err))
	}
}
