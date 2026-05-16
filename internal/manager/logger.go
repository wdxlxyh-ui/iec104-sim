package manager

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

const (
	LogMaxSize  int64 = 10 * 1024 * 1024
	LogMaxFiles int   = 4
)

type InstanceLogger struct {
	mu       sync.Mutex
	logDir   string
	logFile  *os.File
	writer   io.Writer
	instance string
}

func NewInstanceLogger(cfgDir string, instanceID string, port int) (*InstanceLogger, error) {
	logDir := filepath.Join(cfgDir, "logs", fmt.Sprintf("%s-%d", instanceID, port))
	
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	logger := &InstanceLogger{
		logDir:   logDir,
		instance: fmt.Sprintf("%s-%d", instanceID, port),
	}

	if err := logger.openLogFile(); err != nil {
		return nil, err
	}

	return logger, nil
}

func (l *InstanceLogger) openLogFile() error {
	logPath := filepath.Join(l.logDir, "instance.log")
	
	if fi, err := os.Stat(logPath); err == nil && fi.Size() >= LogMaxSize {
		l.rotateLogs()
	}

	f, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	l.logFile = f
	l.writer = f
	return nil
}

func (l *InstanceLogger) rotateLogs() error {
	oldestPath := fmt.Sprintf("instance.log.%d", LogMaxFiles-1)
	if _, err := os.Stat(filepath.Join(l.logDir, oldestPath)); err == nil {
		os.Remove(oldestPath)
	}

	for i := LogMaxFiles - 2; i >= 0; i-- {
		src := fmt.Sprintf("instance.log.%d", i)
		dst := fmt.Sprintf("instance.log.%d", i+1)
		
		srcPath := filepath.Join(l.logDir, src)
		dstPath := filepath.Join(l.logDir, dst)
		
		if _, err := os.Stat(srcPath); err == nil {
			os.Rename(srcPath, dstPath)
		}
	}

	if l.logFile != nil {
		l.logFile.Close()
	}
	
	oldLogPath := filepath.Join(l.logDir, "instance.log")
	newLogPath := filepath.Join(l.logDir, "instance.log.0")
	os.Rename(oldLogPath, newLogPath)

	return l.openLogFile()
}

func (l *InstanceLogger) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.logFile != nil {
		fi, _ := l.logFile.Stat()
		if fi.Size() >= LogMaxSize {
			l.rotateLogs()
		}
	}

	return l.writer.Write(p)
}

func (l *InstanceLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.logFile != nil {
		return l.logFile.Close()
	}
	return nil
}

func (l *InstanceLogger) Handler() slog.Handler {
	return slog.NewTextHandler(l, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
}

func (l *InstanceLogger) GetLogDir() string {
	return l.logDir
}

func RemoveInstanceLogDir(cfgDir string, instanceID string, port int) error {
	logDir := filepath.Join(cfgDir, "logs", fmt.Sprintf("%s-%d", instanceID, port))
	return os.RemoveAll(logDir)
}

func RenameInstanceLogDir(cfgDir string, oldID string, newID string, oldPort int, newPort int) error {
	oldDir := filepath.Join(cfgDir, "logs", fmt.Sprintf("%s-%d", oldID, oldPort))
	newDir := filepath.Join(cfgDir, "logs", fmt.Sprintf("%s-%d", newID, newPort))
	
	if _, err := os.Stat(oldDir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	
	return os.Rename(oldDir, newDir)
}