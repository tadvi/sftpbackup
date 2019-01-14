package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const dateFormat = "2006-01-02"

type Transfer struct {
	Server              string
	Port                int
	Username, Password  string
	RemoteDir, LocalDir string
}

var tr = &Transfer{}

func init() {
	flag.StringVar(&tr.Server, "server", "", "Server DNS name")
	flag.IntVar(&tr.Port, "port", 22, "Server port number")
	flag.StringVar(&tr.Username, "u", "root", "User name")
	flag.StringVar(&tr.Password, "p", "", "Password")

	flag.StringVar(&tr.RemoteDir, "remotedir", "", "Remote directory")
	flag.StringVar(&tr.LocalDir, "localdir", ".", "Local directory")
}

func (tr *Transfer) connect() (*ssh.Client, *sftp.Client, error) {
	var auths []ssh.AuthMethod
	if aconn, err := net.Dial("unix", ""); err == nil {
		auths = append(auths, ssh.PublicKeysCallback(agent.NewClient(aconn).Signers))
	}
	if tr.Password != "" {
		auths = append(auths, ssh.Password(tr.Password))
	}

	config := ssh.ClientConfig{
		User: tr.Username,
		Auth: auths,
	}
	addr := fmt.Sprintf("%s:%d", tr.Server, tr.Port)
	conn, err := ssh.Dial("tcp", addr, &config)
	if err != nil {
		log.Println(fmt.Sprintf("Error unable to connect to [%s]: %v", addr, err))
		return nil, nil, err
	}

	client, err := sftp.NewClient(conn)
	if err != nil {
		log.Println(fmt.Sprintf("Error unable to start sftp subsystem: %v", err))
		return nil, nil, err
	}
	return conn, client, nil
}

// download remote files to local directory.
func (tr *Transfer) download(client *sftp.Client) error {

	files, err := client.ReadDir(tr.RemoteDir)
	if err != nil {
		log.Println(fmt.Sprintf("Error while receiving: %v", err))
		return err
	}

	// DEBUG
	for _, file := range files {
		if !file.IsDir() {
			log.Println("Found file:", file.Name())
		}
	}

	// open local file
	now := time.Now()
	dir := filepath.Join(tr.LocalDir, now.Format(dateFormat))
	if err := os.MkdirAll(dir, 0777); err != nil {
		return err
	}
	log.Println("Created dir", dir)

	//log.Println("Files:", files)
	for _, file := range files {
		if !file.IsDir() {
			local, err := os.Create(filepath.Join(dir, file.Name()))
			if err != nil {
				log.Println(fmt.Sprintf("Error can not open local file: %v", err))
				return err
			}
			defer local.Close()

			// open remote file
			remoteFile := filepath.Join(tr.RemoteDir, file.Name())
			f, err := client.Open(remoteFile)
			if err != nil {
				log.Println(fmt.Sprintf("Error while receiving: %v", err))
				return err
			}
			// perform transfer
			if _, err := io.Copy(local, f); err != nil {
				log.Println(fmt.Sprintf("Error while receiving: %v", err))
				f.Close()
				return err
			}
			f.Close()
			log.Println(fmt.Sprintf("Received '%s'", file.Name()))
		}
	}
	return nil
}

func fatal(err error) {
	log.Fatal(err)
}

func main() {
	flag.Parse()

	log.Println("Connecting:", tr.Server)
	conn, client, err := tr.connect()
	if err != nil {
		fatal(err)
	}
	err = tr.download(client)
	if err != nil {
		fatal(err)
	}

	defer client.Close()
	defer conn.Close()

	log.Println("Backup done.")
}
