package transfer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type Transfer struct {
	client *ssh.Client
}

func New(client *ssh.Client) *Transfer {
	return &Transfer{client: client}
}

func (t *Transfer) Upload(localPath, remotePath string) error {
	sftpClient, err := sftp.NewClient(t.client)
	if err != nil {
		return fmt.Errorf("failed to create sftp client: %w", err)
	}
	defer sftpClient.Close()

	srcFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file %s: %w", localPath, err)
	}
	defer srcFile.Close()

	dstFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file %s: %w", remotePath, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

func (t *Transfer) Download(remotePath, localPath string) error {
	sftpClient, err := sftp.NewClient(t.client)
	if err != nil {
		return fmt.Errorf("failed to create sftp client: %w", err)
	}
	defer sftpClient.Close()

	srcFile, err := sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file %s: %w", remotePath, err)
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create local directory: %w", err)
	}

	dstFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file %s: %w", localPath, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

func (t *Transfer) UploadAndExecute(localScriptPath string, args []string) (string, error) {
	sftpClient, err := sftp.NewClient(t.client)
	if err != nil {
		return "", fmt.Errorf("failed to create sftp client: %w", err)
	}
	defer sftpClient.Close()

	srcFile, err := os.Open(localScriptPath)
	if err != nil {
		return "", fmt.Errorf("failed to open local script %s: %w", localScriptPath, err)
	}
	defer srcFile.Close()

	remoteScriptPath := "/tmp/" + filepath.Base(localScriptPath)
	dstFile, err := sftpClient.Create(remoteScriptPath)
	if err != nil {
		return "", fmt.Errorf("failed to create remote script %s: %w", remoteScriptPath, err)
	}

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		dstFile.Close()
		return "", fmt.Errorf("failed to copy script: %w", err)
	}
	dstFile.Close()

	session, err := t.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	chmodCmd := fmt.Sprintf("chmod +x %s", remoteScriptPath)
	if err := session.Run(chmodCmd); err != nil {
		return "", fmt.Errorf("failed to chmod script: %w", err)
	}
	session.Close()

	session2, err := t.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session2.Close()

	var execCmd string
	if len(args) > 0 {
		execCmd = fmt.Sprintf("%s %s", remoteScriptPath, strings.Join(args, " "))
	} else {
		execCmd = remoteScriptPath
	}

	output, err := session2.CombinedOutput(execCmd)
	if err != nil {
		return string(output), fmt.Errorf("script execution failed: %w", err)
	}

	session3, err := t.client.NewSession()
	if err == nil {
		session3.Run(fmt.Sprintf("rm -f %s", remoteScriptPath))
		session3.Close()
	}

	return string(output), nil
}
