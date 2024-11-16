package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/dmitriykara/word-of-wisdom-pow/internal/config"
	"go.uber.org/zap"
)

const (
	letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	maxDifficultyClientCount = 50
	minDifficultyClientCount = 20
)

var (
	runes = []rune(letters)
)

// WordOfWisdomServer is a server that serves word of wisdom requests
type WordOfWisdomServer struct {
	config     config.ServerConfig
	listener   net.Listener
	quotes     []string
	clientLoad int
	mu         sync.Mutex
	logger     *zap.Logger
}

// NewServer initializes a new server with the given configuration and logger
func NewServer(cfg config.ServerConfig, logger *zap.Logger) *WordOfWisdomServer {
	return &WordOfWisdomServer{
		config: cfg,
		quotes: []string{
			"The only true wisdom is in knowing you know nothing. - Socrates",
			"The journey of a thousand miles begins with one step. - Lao Tzu",
			"That which does not kill us makes us stronger. - Friedrich Nietzsche",
			"Life is what happens when you’re busy making other plans. - John Lennon",
			"When the going gets tough, the tough get going. - Joe Kennedy",
		},
		logger: logger,
	}
}

// Start launches the server and begins accepting connections
func (s *WordOfWisdomServer) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}
	s.listener = listener
	s.logger.Info("Server started", zap.String("address", addr))

	var wg sync.WaitGroup
	connectionChan := make(chan net.Conn)

	for i := 0; i < s.config.MaxConnections; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for conn := range connectionChan {
				s.handleConnection(conn)
			}
		}()
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				break
			}
			s.logger.Error("Error accepting connection", zap.Error(err))

			continue
		}
		select {
		case connectionChan <- conn:
		default:
			s.logger.Warn("Maximum connections reached", zap.String("client", conn.RemoteAddr().String()))

			if err := conn.Close(); err != nil {
				s.logger.Error("conn close error", zap.Error(err))
			}
		}
	}

	close(connectionChan)
	wg.Wait()

	return nil
}

// handleConnection processes a single client connection
func (s *WordOfWisdomServer) handleConnection(conn net.Conn) {
	defer func() {
		_ = conn.Close()
	}()

	clientAddr := conn.RemoteAddr().String()
	s.logger.Info("Accepted connection", zap.String("client", clientAddr))

	s.incrementClientLoad()
	defer s.decrementClientLoad()

	// Generate challenge and difficulty
	difficulty := s.adjustDifficulty()
	challenge := s.generateChallenge()
	serverTimestamp := time.Now().UTC()

	// Send challenge to client
	if err := s.sendChallenge(conn, challenge, serverTimestamp, difficulty); err != nil {
		s.logger.Error("Failed to send challenge", zap.String("client", clientAddr), zap.Error(err))

		return
	}

	// Receive PoW response from client
	nonce, clientTimestamp, err := s.receiveResponse(conn)
	if err != nil {
		s.logger.Error("Failed to receive response", zap.String("client", clientAddr), zap.Error(err))

		return
	}

	// Verify Proof of Work using the original serverTimestamp
	if s.verifyPoW(challenge, nonce, clientTimestamp, serverTimestamp, difficulty) {
		quote := s.getRandomQuote()
		if err := s.sendQuote(conn, quote); err != nil {
			s.logger.Error("Failed to send quote", zap.String("client", clientAddr), zap.Error(err))
		} else {
			s.logger.Info("Quote sent successfully", zap.String("client", clientAddr))
		}
	} else {
		s.sendError(conn, "Invalid proof of work.")
		s.logger.Warn("Invalid PoW attempt", zap.String("client", clientAddr))
	}
}

// incrementClientLoad increases the active client count
func (s *WordOfWisdomServer) incrementClientLoad() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.clientLoad++
}

// decrementClientLoad decreases the active client count
func (s *WordOfWisdomServer) decrementClientLoad() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.clientLoad--
}

// adjustDifficulty dynamically adjusts difficulty based on load
func (s *WordOfWisdomServer) adjustDifficulty() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.clientLoad > maxDifficultyClientCount {
		return s.config.MaxDifficulty
	} else if s.clientLoad > minDifficultyClientCount {
		return s.config.MaxDifficulty - 1
	}

	return s.config.MinDifficulty
}

// generateChallenge creates a unique challenge string
func (s *WordOfWisdomServer) generateChallenge() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	b := make([]rune, 64)
	for i := range b {
		b[i] = runes[r.Intn(len(runes))]
	}

	return string(b)
}

// sendChallenge sends the PoW challenge to the client
func (s *WordOfWisdomServer) sendChallenge(conn net.Conn, challenge string, timestamp time.Time, difficulty int) error {
	message := fmt.Sprintf("Challenge:%s;Timestamp:%s;Difficulty:%d\n",
		challenge, timestamp.Format(time.RFC3339Nano), difficulty)

	_, err := conn.Write([]byte(message))

	return err
}

// receiveResponse reads the client's PoW solution
func (s *WordOfWisdomServer) receiveResponse(conn net.Conn) (string, time.Time, error) {
	buffer := make([]byte, 4096)

	if err := conn.SetReadDeadline(time.Now().Add(s.config.ConnectionTimeout)); err != nil {
		s.logger.Error("set read deadline failed", zap.Error(err))
	}

	n, err := conn.Read(buffer)
	if err != nil {
		return "", time.Time{}, err
	}
	response := strings.TrimSpace(string(buffer[:n]))

	return s.parseResponse(response)
}

// parseResponse extracts the nonce and timestamp from the client's response
func (s *WordOfWisdomServer) parseResponse(response string) (string, time.Time, error) {
	parts := strings.Split(response, ";")
	if len(parts) != 2 {
		return "", time.Time{}, errors.New("invalid response format")
	}

	nonce := strings.TrimPrefix(parts[0], "Nonce:")
	timestampStr := strings.TrimPrefix(parts[1], "Timestamp:")
	timestamp, err := time.Parse(time.RFC3339Nano, timestampStr)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("invalid timestamp format: %w", err)
	}

	return nonce, timestamp, nil
}

// verifyPoW validates the client's PoW solution
func (s *WordOfWisdomServer) verifyPoW(challenge, nonce string, clientTimestamp, serverTimestamp time.Time, difficulty int) bool {
	// Check if the client's timestamp is within the allowed TimeWindow
	if time.Since(clientTimestamp) > s.config.TimeWindow {
		s.logger.Warn("Timestamp expired", zap.Time("client_timestamp", clientTimestamp), zap.Duration("time_window", s.config.TimeWindow))

		return false
	}

	// Use the original serverTimestamp for PoW verification
	data := fmt.Sprintf("%s%s%s", challenge, nonce, serverTimestamp.Format(time.RFC3339Nano))
	s.logger.Debug("Verifying PoW", zap.String("data", data))

	// Compute the hash
	hash := sha256.Sum256([]byte(data))
	hashHex := hex.EncodeToString(hash[:])

	s.logger.Debug("Computed hash", zap.String("hashHex", hashHex))

	// Check if the hash meets the required difficulty
	requiredPrefix := strings.Repeat("0", difficulty)

	return strings.HasPrefix(hashHex, requiredPrefix)
}

// getRandomQuote selects a random quote from the list
func (s *WordOfWisdomServer) getRandomQuote() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	return s.quotes[r.Intn(len(s.quotes))]
}

// sendQuote transmits a quote to the client
func (s *WordOfWisdomServer) sendQuote(conn net.Conn, quote string) error {
	message := fmt.Sprintf("Quote:%s\n", quote)
	_, err := conn.Write([]byte(message))

	return err
}

// sendError notifies the client of an error
func (s *WordOfWisdomServer) sendError(conn net.Conn, errorMessage string) {
	message := fmt.Sprintf("Error:%s\n", errorMessage)

	_, err := conn.Write([]byte(message))
	if err != nil {
		s.logger.Error("send error failed:", zap.Error(err))
	}
}

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

	server := NewServer(cfg.Server, logger)
	if err := server.Start(); err != nil {
		logger.Fatal("Server error", zap.Error(err))
	}
}
