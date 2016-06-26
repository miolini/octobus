package main

import (
	"fmt"
	flag "github.com/ogier/pflag"
	"golang.org/x/crypto/ssh"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/user"
	"strings"
	"sync"
)

func main() {
	flHost := flag.String("hosts", "", "hosts, support @filepath for loading hosts from file")
	flKey := flag.String("key", "", "private key")
	flUser := flag.String("user", "root", "remote user")
	flCmd := flag.String("cmd", "uname -a", "remote command")
	flPass := flag.String("pass", "", "optional user password")
	flVerbose := flag.Bool("verbose", false, "verbose mode (default false)")
	flReconnect := flag.Bool("reconnect", false, "reconnect on disconnected sessions (default false)")
	flag.Parse()

	hosts, err := parseHosts(*flHost)
	if err != nil {
		log.Printf("parse hosts err: %s", err)
		return
	}

	if *flVerbose {
		log.Printf("user: %s", *flUser)
		log.Printf("cmd: %s", *flCmd)
		log.Printf("hosts: %d", len(hosts))
	}

	privateKey, err := loadPrivateKey(*flKey)
	if err != nil {
		log.Fatalf("load private key err: %s", err)
		return
	}

	runCmdOnHosts(hosts, *flCmd, *flUser, *flPass, privateKey, *flReconnect, *flVerbose)
}

func runCmdOnHosts(hosts []string, cmd, defaultUser, defaultPass string, privateKey ssh.AuthMethod, reconnect, verbose bool) {
	wg := sync.WaitGroup{}
	wg.Add(len(hosts))
	for _, host := range hosts {
		go runCmdOnHost(host, cmd, defaultUser, defaultPass, privateKey, reconnect, verbose, &wg)
	}
	wg.Wait()
}

func runCmdOnHost(host string, cmd, defaultUser, defaultPass string, privateKey ssh.AuthMethod, reconnect, verbose bool, wg *sync.WaitGroup) {
	defer wg.Done()
	hostParsed, err := url.Parse(host)
	if err != nil {
		log.Printf("host %s parse failed: %s", host, err)
		return
	}
	var user string
	var pass string
	if hostParsed.Host == "" {
		hostParsed.Host = host
	}
	hostParts := strings.Split(hostParsed.Host, ":")
	if len(hostParts) == 1 {
		host = hostParts[0] + ":22"
	} else {
		host = hostParts[0] + ":" + hostParts[1]
	}
	if hostParsed.User != nil {
		user = hostParsed.User.Username()
		pass, _ = hostParsed.User.Password()
	}
	if user == "" {
		user = defaultUser
	}
	if pass == "" {
		pass = defaultPass
	}
	if verbose {
		log.Printf("connect to %s:%s@%s", user, pass, host)
	}
	clientConfig := ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			privateKey,
		},
	}
	if pass != "" {
		clientConfig.Auth = append([]ssh.AuthMethod{ssh.Password(pass)}, clientConfig.Auth...)
	}
	for {
		client, err := ssh.Dial("tcp", host, &clientConfig)
		if err != nil {
			log.Printf("connect to %s err: %s", host, err)
			if reconnect {
				continue
			} else {
				return
			}
		}
		session, err := client.NewSession()
		if err != nil {
			log.Printf("session for %s failed: %s", host, err)
			if reconnect {
				continue
			} else {
				return
			}
		}
		defer session.Close()
		stderr, err := session.StderrPipe()
		if err != nil {
			log.Printf("pipe stderr err: %s", err)
			if reconnect {
				continue
			} else {
				return
			}
		}
		stdout, err := session.StdoutPipe()
		if err != nil {
			log.Printf("pipe stdout err: %s", err)
			if reconnect {
				continue
			} else {
				return
			}
		}
		go io.Copy(&safeWriter{W: os.Stdout}, stdout)
		go io.Copy(&safeWriter{W: os.Stderr}, stderr)
		if err := session.Run(cmd); err != nil {
			log.Printf("failed execute on host %s: %s", host, err)
			if _, ok := err.(*ssh.ExitError); !ok && reconnect {
				continue
			}
			return
		}
		break
	}
}

type needReconnect struct {
	err error
}

type safeWriter struct {
	W     io.Writer
	mutex sync.Mutex
}

func (sw *safeWriter) Write(data []byte) (int, error) {
	sw.mutex.Lock()
	n, err := sw.W.Write(data)
	sw.mutex.Unlock()
	return n, err
}

func loadPrivateKey(keyPath string) (ssh.AuthMethod, error) {
	if keyPath == "" {
		return nil, fmt.Errorf("need private key")
	}
	if keyPath[:2] == "~/" {
		usr, _ := user.Current()
		keyPath = strings.Replace(keyPath, "~", usr.HomeDir, 1)
	}
	keyData, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeys(signer), nil
}

func parseHosts(flagHost string) ([]string, error) {
	if strings.HasPrefix(flagHost, "@") {
		hostData, err := ioutil.ReadFile(flagHost)
		if err != nil {
			return nil, err
		}
		return strings.Split(string(hostData), "\n"), nil
	}
	return strings.Split(flagHost, ","), nil
}
