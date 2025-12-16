package models

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"

	"github.com/astaxie/beego"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func SshOneCommand(client *ssh.Client, command string) ([]byte, error) {
	beego.Info(fmt.Sprintf("Execute command on [%s]: %s", client.Conn.RemoteAddr(), command))

	session, err := client.NewSession()
	if err != nil {
		outErr := fmt.Errorf("create ssh session fail: %w", err)
		beego.Error(outErr)
		return nil, outErr
	}
	defer session.Close()

	// CombinedOutput lấy cả stdout và stderr
	output, err := session.CombinedOutput(command)

	if err != nil {
		beego.Error(fmt.Sprintf("ssh execute failed [%s]: %v", command, err))
		beego.Error(fmt.Sprintf("ssh output:\n%s", string(output)))
		return output, fmt.Errorf("remote command [%s] failed: %w", command, err)
	}

	// Khi thành công
	beego.Info(fmt.Sprintf("ssh execute success [%s]:\n%s", command, string(output)))
	return output, nil
}

// create an ssh client with password
func SshClientWithPasswd(user, passwd, ip string, port int) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		Timeout:         SshTimeout,
		User:            user,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth:            []ssh.AuthMethod{ssh.Password(passwd)},
	}
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", ip, port), config)
	if err != nil {
		outErr := fmt.Errorf("create ssh client fail: error: %w", err)
		beego.Error(outErr)
		return nil, outErr
	}
	return client, err
}

// create an ssh client with pem private key identity file
func SshClientWithPem(pemFilePath, user, ip string, port int) (*ssh.Client, error) {
	pemByte, err := ioutil.ReadFile(strings.TrimSpace(pemFilePath))
	if err != nil {
		return nil, fmt.Errorf("read ssh private key file %s error: %w", pemFilePath, err)
	}

	signer, err := ssh.ParsePrivateKey(pemByte)
	if err != nil {
		return nil, fmt.Errorf("ssh.ParsePrivateKey error: %w", err)
	}

	fp := ssh.FingerprintSHA256(signer.PublicKey())
	beego.Info(fmt.Sprintf("[SSH] connecting %s@%s:%d with key fp=%s (from %s)",
		user, ip, port, fp, pemFilePath))

	// Hỗ trợ ssh-agent + (tùy chọn) password fallback
	auth := []ssh.AuthMethod{ssh.PublicKeys(signer)}
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if conn, err := net.Dial("unix", sock); err == nil {
			agentClient := agent.NewClient(conn)
			auth = append(auth, ssh.PublicKeysCallback(agentClient.Signers))
		}
	}
	if pw := beego.AppConfig.String("k8sVmSshPassword"); pw != "" {
		auth = append(auth, ssh.Password(pw))
	}

	config := &ssh.ClientConfig{
		Timeout:         SshTimeout,
		User:            user,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", ip, port), config)
	if err != nil {
		return nil, fmt.Errorf("ssh.Dial error: %w", err)
	}
	return client, nil
}

func SftpCopyFile(srcPath, dstPath string, sshClient *ssh.Client) error {
	beego.Info(fmt.Sprintf("SFTP copy file [local:%s] to [%s:%s].", srcPath, sshClient.Conn.RemoteAddr(), dstPath))

	// open an SFTP session over an existing ssh connection.
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		if err != nil {
			outErr := fmt.Errorf("create SFTP client, error: %w", err)
			beego.Error(outErr)
			return outErr
		}
	}
	defer sftpClient.Close()

	// Open the source file
	srcFile, err := os.Open(srcPath)
	if err != nil {
		outErr := fmt.Errorf("open source file %s, error: %w", srcPath, err)
		beego.Error(outErr)
		return outErr
	}
	defer srcFile.Close()

	// Create the destination file
	dstFile, err := sftpClient.Create(dstPath)
	if err != nil {
		outErr := fmt.Errorf("create the destination file %s:%s, error: %w", sshClient.Conn.RemoteAddr(), dstPath, err)
		beego.Error(outErr)
		return outErr
	}
	defer dstFile.Close()

	// write from source file to destination file
	if _, err := dstFile.ReadFrom(srcFile); err != nil {
		outErr := fmt.Errorf("write from source file %s to the destination file %s:%s, error: %w", srcPath, sshClient.Conn.RemoteAddr(), dstPath, err)
		beego.Error(outErr)
		return outErr
	}

	return nil
}
