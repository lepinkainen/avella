package actions

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"

	"github.com/lepinkainen/avella/ssh"
	"github.com/lepinkainen/avella/template"
)

// SCPAction uploads a file to a remote host via SFTP.
type SCPAction struct {
	Host         string
	Dest         string
	Pool         *ssh.Pool
	DeleteSource bool
}

func (a *SCPAction) String() string { return fmt.Sprintf("scp → %s:%s", a.Host, a.Dest) }

// Describe returns the resolved destination for a specific file.
func (a *SCPAction) Describe(filePath string) string {
	dest, err := template.ResolveDest(a.Dest, filePath)
	if err != nil {
		return a.String()
	}
	return fmt.Sprintf("scp → %s:%s", a.Host, dest)
}

// Execute uploads the file at path to the remote destination via SFTP.
func (a *SCPAction) Execute(_ context.Context, filePath string) (retErr error) {
	destDir, err := template.ResolveDest(a.Dest, filePath)
	if err != nil {
		return fmt.Errorf("resolve dest for %s: %w", filePath, err)
	}

	sftpClient, err := a.Pool.SFTP(a.Host)
	if err != nil {
		return fmt.Errorf("SFTP connect %s: %w", a.Host, err)
	}
	defer func() {
		if closeErr := sftpClient.Close(); closeErr != nil && retErr == nil {
			retErr = closeErr
		}
	}()

	src, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open %s: %w", filePath, err)
	}

	srcInfo, err := src.Stat()
	if err != nil {
		_ = src.Close()
		return fmt.Errorf("stat %s: %w", filePath, err)
	}

	remotePath := path.Join(destDir, srcInfo.Name())

	dst, err := sftpClient.Create(remotePath)
	if err != nil {
		_ = src.Close()
		return fmt.Errorf("create remote %s: %w", remotePath, err)
	}

	written, err := io.Copy(dst, src)

	// Close both files before checking errors or deleting.
	if closeErr := dst.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if closeErr := src.Close(); closeErr != nil && err == nil {
		err = closeErr
	}

	if err != nil {
		return fmt.Errorf("upload %s to %s:%s: %w", filePath, a.Host, remotePath, err)
	}

	if written != srcInfo.Size() {
		return fmt.Errorf("size mismatch: local=%d written=%d", srcInfo.Size(), written)
	}

	remoteDest := fmt.Sprintf("%s:%s", a.Host, remotePath)

	if a.DeleteSource {
		if removeErr := os.Remove(filePath); removeErr != nil {
			return fmt.Errorf("uploaded to %s but failed to delete source: %w", remoteDest, removeErr)
		}
		slog.Info("uploaded and deleted file", "src", filePath, "dest", remoteDest, "bytes", written)
	} else {
		slog.Info("uploaded file", "src", filePath, "dest", remoteDest, "bytes", written)
	}

	return nil
}
