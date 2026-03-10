package actions

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"

	"github.com/lepinkainen/avella/ssh"
)

// SCPAction uploads a file to a remote host via SFTP.
type SCPAction struct {
	Host string
	Dest string
	Pool *ssh.Pool
}

func (a *SCPAction) String() string { return fmt.Sprintf("scp → %s:%s", a.Host, a.Dest) }

// Execute uploads the file at path to the remote destination via SFTP.
func (a *SCPAction) Execute(_ context.Context, filePath string) (retErr error) {
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
	defer func() {
		if closeErr := src.Close(); closeErr != nil && retErr == nil {
			retErr = closeErr
		}
	}()

	srcInfo, err := src.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", filePath, err)
	}

	remotePath := path.Join(a.Dest, srcInfo.Name())

	dst, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("create remote %s: %w", remotePath, err)
	}
	defer func() {
		if closeErr := dst.Close(); closeErr != nil && retErr == nil {
			retErr = closeErr
		}
	}()

	written, err := io.Copy(dst, src)
	if err != nil {
		return fmt.Errorf("upload %s to %s:%s: %w", filePath, a.Host, remotePath, err)
	}

	if written != srcInfo.Size() {
		return fmt.Errorf("size mismatch: local=%d written=%d", srcInfo.Size(), written)
	}

	slog.Info("uploaded file",
		"src", filePath,
		"dest", fmt.Sprintf("%s:%s", a.Host, remotePath),
		"bytes", written,
	)
	return nil
}
