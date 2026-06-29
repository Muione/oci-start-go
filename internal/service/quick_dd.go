// Package service -- quick_dd.go: Phase 13.2 Quick DD One-Click Reinstall.
// Provides one-click OS reinstallation via SSH + dd command with real-time
// SSE (Server-Sent Events) progress streaming. Supports downloading OS images
// from URLs and writing them to the instance's boot disk via dd.
package service

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/crypto/ssh"

	"github.com/Muione/oci-start-go/internal/db"
	"github.com/Muione/oci-start-go/internal/repo"
)

// QuickDDService manages quick DD reinstallation operations.
type QuickDDService struct {
	store  *db.Store
	logger zerolog.Logger
}

func NewQuickDDService(store *db.Store, logger zerolog.Logger) *QuickDDService {
	return &QuickDDService{store: store, logger: logger}
}

// DDRequest is the input for a quick DD operation.
type DDRequest struct {
	InstanceID int64  `json:"instanceId"`
	ImageURL   string `json:"imageUrl"`   // URL of the OS image to dd
	TargetDisk string `json:"targetDisk"` // target disk device (default: /dev/sda)
	BlockSize  string `json:"blockSize"`  // dd bs= parameter (default: 4M)
	Gunzip     bool   `json:"gunzip"`     // pipe through gunzip
}

// DDProgress is a single SSE progress event.
type DDProgress struct {
	Status    string  `json:"status"`    // "connecting", "downloading", "writing", "verifying", "completed", "error"
	Percent   float64 `json:"percent"`   // 0-100
	Speed     string  `json:"speed"`     // human-readable speed
	ETA       string  `json:"eta"`       // estimated time remaining
	Message   string  `json:"message"`   // human-readable message
	Error     string  `json:"error,omitempty"`
	Timestamp string  `json:"timestamp"`
}

// DDResult is the final result of a DD operation.
type DDResult struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	Duration  string `json:"duration"`
	ImageURL  string `json:"imageUrl"`
	TargetDisk string `json:"targetDisk"`
}

// ValidateDDRequest validates the DD request parameters.
func (s *QuickDDService) ValidateDDRequest(req *DDRequest) error {
	if req.InstanceID <= 0 {
		return fmt.Errorf("instanceId is required")
	}
	if req.ImageURL == "" {
		return fmt.Errorf("imageUrl is required")
	}
	if req.TargetDisk == "" {
		req.TargetDisk = "/dev/sda"
	}
	if req.BlockSize == "" {
		req.BlockSize = "4M"
	}
	return nil
}

// GetInstanceSSHConfig retrieves SSH connection details for an instance.
func (s *QuickDDService) GetInstanceSSHConfig(ctx context.Context, instanceID int64) (host string, port int, username, password string, err error) {
	inst, err := repo.New(s.store.Read).FindInstanceDetailByID(ctx, instanceID)
	if err != nil {
		return "", 0, "", "", fmt.Errorf("find instance %d: %w", instanceID, err)
	}

	ip := nsStr(inst.PublicIps)
	if ip == "" {
		return "", 0, "", "", fmt.Errorf("instance %d has no public IP", instanceID)
	}

	port = int(ni64(inst.Port))
	if port <= 0 {
		port = 22
	}

	username = nsStr(inst.Username)
	if username == "" {
		username = "root"
	}

	password = nsStr(inst.Password)
	if password == "" {
		return "", 0, "", "", fmt.Errorf("instance %d has no SSH password configured", instanceID)
	}

	return ip, port, username, password, nil
}

// RunDDWithProgress executes the DD operation via SSH and streams progress
// events through the provided channel. The caller is responsible for closing
// the channel when the function returns.
func (s *QuickDDService) RunDDWithProgress(ctx context.Context, req DDRequest, progressCh chan<- DDProgress) error {
	start := time.Now()

	// Validate request.
	if err := s.ValidateDDRequest(&req); err != nil {
		s.sendProgress(progressCh, DDProgress{
			Status:    "error",
			Error:     err.Error(),
			Message:   "Validation failed",
			Timestamp: nowStr(),
		})
		return err
	}

	// Get SSH config.
	s.sendProgress(progressCh, DDProgress{
		Status:    "connecting",
		Message:   "Retrieving SSH configuration...",
		Timestamp: nowStr(),
	})

	host, port, username, password, err := s.GetInstanceSSHConfig(ctx, req.InstanceID)
	if err != nil {
		s.sendProgress(progressCh, DDProgress{
			Status:    "error",
			Error:     err.Error(),
			Message:   "Failed to get SSH config",
			Timestamp: nowStr(),
		})
		return err
	}

	// Connect via SSH.
	s.sendProgress(progressCh, DDProgress{
		Status:    "connecting",
		Message:   fmt.Sprintf("Connecting to %s:%d...", host, port),
		Timestamp: nowStr(),
	})

	sshClient, err := s.connectSSH(host, port, username, password)
	if err != nil {
		s.sendProgress(progressCh, DDProgress{
			Status:    "error",
			Error:     err.Error(),
			Message:   "SSH connection failed",
			Timestamp: nowStr(),
		})
		return fmt.Errorf("SSH connect: %w", err)
	}
	defer sshClient.Close()

	s.sendProgress(progressCh, DDProgress{
		Status:    "connecting",
		Message:   "SSH connected successfully",
		Timestamp: nowStr(),
	})

	// Build the dd command.
	ddCmd := s.buildDDCommand(req)
	s.logger.Info().Str("cmd", ddCmd).Int64("instance", req.InstanceID).Msg("quick_dd: executing")

	s.sendProgress(progressCh, DDProgress{
		Status:    "downloading",
		Message:   "Starting DD operation...",
		Timestamp: nowStr(),
	})

	// Execute the command.
	session, err := sshClient.NewSession()
	if err != nil {
		s.sendProgress(progressCh, DDProgress{
			Status:    "error",
			Error:     err.Error(),
			Message:   "Failed to create SSH session",
			Timestamp: nowStr(),
		})
		return fmt.Errorf("SSH session: %w", err)
	}
	defer session.Close()

	// Get a pipe for stderr (pv/dd progress goes to stderr).
	stderr, err := session.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	// Start the command.
	if err := session.Start(ddCmd); err != nil {
		s.sendProgress(progressCh, DDProgress{
			Status:    "error",
			Error:     err.Error(),
			Message:   "Failed to start DD command",
			Timestamp: nowStr(),
		})
		return fmt.Errorf("start command: %w", err)
	}

	// Parse progress from stderr (pv output or dd output).
	go s.parseProgress(ctx, stderr, progressCh)

	// Wait for command to finish.
	err = session.Wait()
	duration := time.Since(start)

	if err != nil {
		s.sendProgress(progressCh, DDProgress{
			Status:    "error",
			Error:     err.Error(),
			Message:   "DD command failed",
			Timestamp: nowStr(),
		})
		return fmt.Errorf("DD command: %w", err)
	}

	// Success.
	s.sendProgress(progressCh, DDProgress{
		Status:    "completed",
		Percent:   100,
		Message:   fmt.Sprintf("DD completed in %s", duration.Round(time.Second)),
		Timestamp: nowStr(),
	})

	return nil
}

// RunDD executes the DD operation and returns the final result.
func (s *QuickDDService) RunDD(ctx context.Context, req DDRequest) (*DDResult, error) {
	start := time.Now()

	if err := s.ValidateDDRequest(&req); err != nil {
		return nil, err
	}

	host, port, username, password, err := s.GetInstanceSSHConfig(ctx, req.InstanceID)
	if err != nil {
		return nil, err
	}

	sshClient, err := s.connectSSH(host, port, username, password)
	if err != nil {
		return nil, fmt.Errorf("SSH connect: %w", err)
	}
	defer sshClient.Close()

	ddCmd := s.buildDDCommand(req)
	session, err := sshClient.NewSession()
	if err != nil {
		return nil, fmt.Errorf("SSH session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(ddCmd)
	duration := time.Since(start)

	if err != nil {
		return &DDResult{
			Success:    false,
			Message:    fmt.Sprintf("DD failed: %v\nOutput: %s", err, string(output)),
			Duration:   duration.Round(time.Second).String(),
			ImageURL:   req.ImageURL,
			TargetDisk: req.TargetDisk,
		}, err
	}

	return &DDResult{
		Success:    true,
		Message:    "DD completed successfully",
		Duration:   duration.Round(time.Second).String(),
		ImageURL:   req.ImageURL,
		TargetDisk: req.TargetDisk,
	}, nil
}

// RebootInstance reboots the instance via SSH after DD completion.
func (s *QuickDDService) RebootInstance(ctx context.Context, instanceID int64) error {
	host, port, username, password, err := s.GetInstanceSSHConfig(ctx, instanceID)
	if err != nil {
		return err
	}

	sshClient, err := s.connectSSH(host, port, username, password)
	if err != nil {
		return fmt.Errorf("SSH connect: %w", err)
	}
	defer sshClient.Close()

	session, err := sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("SSH session: %w", err)
	}
	defer session.Close()

	// Execute reboot. Don't wait for it to complete since the connection will drop.
	_ = session.Start("reboot")
	return nil
}

// --- Internal helpers ---

func (s *QuickDDService) connectSSH(host string, port int, username, password string) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("SSH dial %s: %w", addr, err)
	}
	return client, nil
}

func (s *QuickDDService) buildDDCommand(req DDRequest) string {
	var cmd strings.Builder

	if req.Gunzip {
		// curl | gunzip | dd
		cmd.WriteString(fmt.Sprintf("curl -sL '%s' | gunzip | dd of='%s' bs='%s' status=progress 2>&1",
			req.ImageURL, req.TargetDisk, req.BlockSize))
	} else {
		// curl | dd
		cmd.WriteString(fmt.Sprintf("curl -sL '%s' | dd of='%s' bs='%s' status=progress 2>&1",
			req.ImageURL, req.TargetDisk, req.BlockSize))
	}

	return cmd.String()
}

func (s *QuickDDService) parseProgress(ctx context.Context, r io.Reader, progressCh chan<- DDProgress) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		progress := s.parseDDLine(line)
		if progress != nil {
			progress.Timestamp = nowStr()
			s.sendProgress(progressCh, *progress)
		}
	}
}

// parseDDLine parses dd status=progress output lines.
// dd outputs lines like: "123456789 bytes (123 MB, 118 MiB) copied, 10 s, 12.3 MB/s"
// or with pv: "  150MiB  0:00:10 [15.0MiB/s] [============>     ] 60% ETA 0:00:07"
func (s *QuickDDService) parseDDLine(line string) *DDProgress {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	// Try to parse pv output (has percentage).
	if strings.Contains(line, "%") {
		return s.parsePVLine(line)
	}

	// Try to parse dd output (has "bytes" or "copied").
	if strings.Contains(line, "copied") || strings.Contains(line, "bytes") {
		return &DDProgress{
			Status:  "writing",
			Message: line,
		}
	}

	return nil
}

// parsePVLine parses pv (pipe viewer) progress output.
// Example: "  150MiB  0:00:10 [15.0MiB/s] [============>     ] 60% ETA 0:00:07"
func (s *QuickDDService) parsePVLine(line string) *DDProgress {
	progress := &DDProgress{
		Status:  "writing",
		Message: line,
	}

	// Extract percentage.
	parts := strings.Fields(line)
	for _, p := range parts {
		if strings.HasSuffix(p, "%") {
			pctStr := strings.TrimSuffix(p, "%")
			var pct float64
			if _, err := fmt.Sscanf(pctStr, "%f", &pct); err == nil {
				progress.Percent = pct
			}
		}
	}

	// Extract speed (in brackets).
	if idx := strings.Index(line, "["); idx >= 0 {
		endIdx := strings.Index(line[idx:], "]")
		if endIdx > 0 {
			speed := line[idx+1 : idx+endIdx]
			if strings.Contains(speed, "/s") {
				progress.Speed = speed
			}
		}
	}

	// Extract ETA.
	if etaIdx := strings.Index(line, "ETA"); etaIdx >= 0 {
		eta := strings.TrimSpace(line[etaIdx+3:])
		if eta != "" {
			progress.ETA = eta
		}
	}

	return progress
}

func (s *QuickDDService) sendProgress(ch chan<- DDProgress, p DDProgress) {
	select {
	case ch <- p:
	default:
		// Channel full, skip this update (non-blocking).
	}
}
