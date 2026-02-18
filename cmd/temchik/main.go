package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/bludnic/temchik"
	"github.com/bludnic/temchik/internal/appdata"
	"github.com/bludnic/temchik/internal/db"
	"github.com/bludnic/temchik/internal/logging"
	"github.com/bludnic/temchik/internal/server"
	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	root := &cobra.Command{
		Use:   "temchik",
		Short: "Temchik (Go rewrite, WIP)",
	}
	root.Version = version

	root.AddCommand(
		newInitCmd(),
		newSetPasswordCmd(),
		newUpCmd(),
		newDownCmd(),
		newStatusCmd(),
		newLogsCmd(),
		newDaemonCmd(),
		newStubCmd("trade", "Live trading (not ported yet)"),
		newStubCmd("backtest", "Backtesting (not ported yet)"),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize ~/.temchik (password, SQLite DB, migrations, seed)",
		RunE: func(_ *cobra.Command, _ []string) error {
			appDir, err := appdata.EnsureDir()
			if err != nil {
				return err
			}

			strategiesDir := filepath.Join(appDir, "strategies")
			if err := os.MkdirAll(strategiesDir, 0o755); err != nil {
				return err
			}

			passPath, err := appdata.PassPath()
			if err != nil {
				return err
			}
			if _, err := os.Stat(passPath); errors.Is(err, os.ErrNotExist) {
				password, err := generatePassword()
				if err != nil {
					return err
				}
				if err := appdata.WritePassword(password); err != nil {
					return err
				}
				fmt.Printf("Generated ADMIN PASSWORD in %s\n", passPath)
				fmt.Printf("Password: %s\n", password)
			}

			migrationsFS, err := temchik.MigrationsFS()
			if err != nil {
				return err
			}

			sqlDB, err := db.Open()
			if err != nil {
				return err
			}
			defer sqlDB.Close()

			if err := db.ApplyPrismaMigrations(sqlDB, migrationsFS); err != nil {
				return err
			}
			if err := db.Seed(sqlDB); err != nil {
				return err
			}

			fmt.Println("Initialization complete.")
			return nil
		},
	}
	return cmd
}

func newSetPasswordCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-password <password>",
		Short: "Set admin password (stored in ~/.temchik/pass)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := appdata.WritePassword(args[0]); err != nil {
				return err
			}
			fmt.Println("Password saved successfully.")
			return nil
		},
	}
	return cmd
}

func generatePassword() (string, error) {
	// Human-friendly-ish: 3x 5 chars + 2 digits, e.g. "k9p3a-f0x1z-m2n8q42"
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	randomPart := func(n int) (string, error) {
		b := make([]byte, n)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		for i := range b {
			b[i] = letters[int(b[i])%len(letters)]
		}
		return string(b), nil
	}

	p1, err := randomPart(5)
	if err != nil {
		return "", err
	}
	p2, err := randomPart(5)
	if err != nil {
		return "", err
	}
	p3, err := randomPart(5)
	if err != nil {
		return "", err
	}

	d, err := randomPart(2)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s-%s-%s%s", p1, p2, p3, d), nil
}

func newUpCmd() *cobra.Command {
	var detach bool
	var host string
	var port int

	cmd := &cobra.Command{
		Use:   "up",
		Short: "Start the Temchik daemon",
		RunE: func(_ *cobra.Command, _ []string) error {
			if pid, _ := appdata.ReadPID(); pid > 0 && processAlive(pid) {
				fmt.Printf("Temchik already running [PID: %d]\n", pid)
				return nil
			}

			if err := appdata.WriteSettings(appdata.Settings{Host: host, Port: port}); err != nil {
				return err
			}

			if detach {
				return startDetachedDaemon()
			}
			return runDaemon(true)
		},
	}

	cmd.Flags().BoolVarP(&detach, "detach", "d", false, "Run in detached mode")
	cmd.Flags().StringVar(&host, "host", appdata.DefaultSettings.Host, "Daemon host")
	cmd.Flags().IntVarP(&port, "port", "p", appdata.DefaultSettings.Port, "Daemon port")
	return cmd
}

func newDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "daemon",
		Short:  "Run daemon process (internal)",
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runDaemon(false)
		},
	}
	return cmd
}

func startDetachedDaemon() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer devNull.Close()

	cmd := exec.Command(exe, "daemon")
	cmd.Stdin = devNull
	cmd.Stdout = devNull
	cmd.Stderr = devNull
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return err
	}

	fmt.Printf("Temchik started as a daemon [PID: %d]\n", cmd.Process.Pid)
	return nil
}

func newDownCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "down",
		Short: "Stop the Temchik daemon",
		RunE: func(_ *cobra.Command, _ []string) error {
			pid, err := appdata.ReadPID()
			if err != nil || pid <= 0 || !processAlive(pid) {
				fmt.Println("Temchik already stopped.")
				_ = appdata.ClearPID()
				return nil
			}

			sig := syscall.SIGTERM
			if force {
				sig = syscall.SIGKILL
			}

			if err := syscall.Kill(pid, sig); err != nil {
				fmt.Printf("Failed to stop Temchik [PID: %d]: %v\n", pid, err)
				fmt.Println("Retry with: temchik down --force")
			} else {
				fmt.Printf("Temchik stopped [PID: %d]\n", pid)
			}

			_ = appdata.ClearPID()
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force kill the daemon process")
	return cmd
}

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show daemon status",
		RunE: func(_ *cobra.Command, _ []string) error {
			pid, err := appdata.ReadPID()
			if err == nil && pid > 0 && processAlive(pid) {
				fmt.Printf("Status: Running [PID: %d]\n", pid)
				return nil
			}
			fmt.Println("Status: Stopped")
			return nil
		},
	}
	return cmd
}

func newLogsCmd() *cobra.Command {
	var follow bool

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Print daemon logs (~/.temchik/log.log)",
		RunE: func(_ *cobra.Command, _ []string) error {
			logPath, err := appdata.LogPath()
			if err != nil {
				return err
			}
			if _, err := os.Stat(logPath); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					fmt.Println("Log file does not exist. Nothing to show.")
					return nil
				}
				return err
			}

			if follow {
				return tailFollow(logPath, 10)
			}

			data, err := os.ReadFile(logPath)
			if err != nil {
				return err
			}
			fmt.Print(string(data))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow logs")
	return cmd
}

func newStubCmd(name, short string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return fmt.Errorf("%s is not implemented yet in the Go rewrite", cmd.Name())
		},
	}
}

func runDaemon(logToStdout bool) error {
	settings, err := appdata.ReadSettings()
	if err != nil {
		return err
	}

	adminPassword, err := appdata.ReadPassword()
	if err != nil {
		passPath, _ := appdata.PassPath()
		return fmt.Errorf("admin password not set. Create %s via: temchik set-password <password>", passPath)
	}

	if _, err := appdata.EnsureDir(); err != nil {
		return err
	}
	logPath, err := appdata.LogPath()
	if err != nil {
		return err
	}
	_, closeLog, err := logging.Init(logPath, logToStdout)
	if err != nil {
		return err
	}
	defer closeLog()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	_ = appdata.WritePID(os.Getpid())
	defer func() { _ = appdata.ClearPID() }()

	slog.Info("Starting Temchik daemon", "host", settings.Host, "port", settings.Port)

	cfg := server.Config{
		Host:          settings.Host,
		Port:          settings.Port,
		AdminPassword: adminPassword,
	}
	return server.Run(ctx, cfg)
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if err := syscall.Kill(pid, 0); err != nil {
		return false
	}
	return true
}

func tailFollow(filePath string, lastLines int) error {
	if lastLines <= 0 {
		lastLines = 10
	}

	printLast := func() error {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		lines := strings.Split(string(data), "\n")
		start := len(lines) - lastLines
		if start < 0 {
			start = 0
		}
		for _, line := range lines[start:] {
			if strings.TrimSpace(line) == "" {
				continue
			}
			fmt.Println(line)
		}
		return nil
	}

	if err := printLast(); err != nil {
		return err
	}

	var lastSize int64
	if st, err := os.Stat(filePath); err == nil {
		lastSize = st.Size()
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		st, err := os.Stat(filePath)
		if err != nil {
			return err
		}
		if st.Size() <= lastSize {
			continue
		}

		f, err := os.Open(filePath)
		if err != nil {
			return err
		}
		if _, err := f.Seek(lastSize, io.SeekStart); err != nil {
			_ = f.Close()
			return err
		}

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}
			fmt.Println(line)
		}
		lastSize = st.Size()
		_ = f.Close()

		if err := scanner.Err(); err != nil {
			return err
		}
	}

	return nil
}
