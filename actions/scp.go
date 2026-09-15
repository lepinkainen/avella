package actions

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"

	"github.com/pkg/sftp"

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
func (a *SCPAction) Execute(ctx context.Context, filePath string) (retErr error) {
	destDir, err := template.ResolveDest(a.Dest, filePath)
	if err != nil {
		return fmt.Errorf("resolve dest for %s: %w", filePath, err)
	}

	sftpClient, err := a.Pool.SFTP(ctx, a.Host)
	if err != nil {
		return fmt.Errorf("SFTP connect %s: %w", a.Host, err)
	}
	defer func() {
		if closeErr := sftpClient.Close(); closeErr != nil && retErr == nil {
			retErr = closeErr
		}
	}()

	// io.Copy over SFTP watches no context, so cancellation has to reach the
	// transfer some other way: closing the session unblocks an in-flight or
	// stalled copy. This client is created per call and the pooled SSH
	// connection underneath it survives, so only this upload is affected.
	transferDone := make(chan struct{})
	defer close(transferDone)
	go func() {
		select {
		case <-ctx.Done():
			_ = sftpClient.Close()
		case <-transferDone:
		}
	}()

	remotePath, written, err := uploadFile(sftpClient, destDir, filePath)
	if err != nil {
		// A cancelled transfer surfaces as some I/O error from the closed
		// session; report why it actually stopped.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("upload %s to %s cancelled: %w", filePath, a.Host, ctxErr)
		}
		return fmt.Errorf("upload %s to %s: %w", filePath, a.Host, err)
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

// uploadFile copies filePath into destDir on the remote host, returning the
// remote path written to and the number of bytes transferred. Both files are
// closed before it returns, so the caller may safely delete the source.
func uploadFile(client *sftp.Client, destDir, filePath string) (remotePath string, written int64, err error) {
	src, err := os.Open(filePath)
	if err != nil {
		return "", 0, fmt.Errorf("open %s: %w", filePath, err)
	}
	defer func() {
		if closeErr := src.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	srcInfo, err := src.Stat()
	if err != nil {
		return "", 0, fmt.Errorf("stat %s: %w", filePath, err)
	}

	remotePath = path.Join(destDir, srcInfo.Name())

	dst, err := client.Create(remotePath)
	if err != nil {
		return "", 0, fmt.Errorf("create remote %s: %w", remotePath, err)
	}

	written, err = io.Copy(dst, src)

	// Close the remote file before checking errors — the close is what flushes
	// the final SFTP writes, so it can surface a failure io.Copy did not.
	if closeErr := dst.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		return "", 0, fmt.Errorf("write %s: %w", remotePath, err)
	}

	if written != srcInfo.Size() {
		return "", 0, fmt.Errorf("size mismatch: local=%d written=%d", srcInfo.Size(), written)
	}

	return remotePath, written, nil
}
