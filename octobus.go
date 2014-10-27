package main

import (
	"code.google.com/p/go.crypto/ssh"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"os/user"
	"strings"
	"sync"
	"time"
)

var (
	flHosts = flag.String("h", "", "host list")
	flFiles = flag.String("f", "", "file list")
	flKey   = flag.String("k", "~/.ssh/id_rsa", "ssh key path")
	flSudo  = flag.Bool("s", false, "use sudo")
	flUser  = flag.String("u", "", "default username")
)

func main() {
	flag.Parse()

	if *flHosts == "" || *flFiles == "" {
		flag.PrintDefaults()
		return
	}

	var err error
	var key ssh.Signer

	if key, err = getKeyFile(*flKey); err != nil {
		log.Fatalf("remote run err: %s", err)
	}

	config := ssh.ClientConfig{
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
	}
	rr := NewRemoteRunner()
	flHostsParts := strings.Split(*flHosts, ",")
	hosts := make([]RemoteHost, len(flHostsParts))
	for i, flHost := range flHostsParts {
		host, err := NewRemoteHostTcp4(flHost, config)
		if err != nil {
			log.Fatalf("error: %s", err)
		}
		if *flUser != "" {
			host.Config.User = *flUser
		}
		hosts[i] = host
	}
	if err = rr.TailForever(*flFiles, *flSudo, hosts...); err != nil {
		log.Fatalf("remote run err: %s", err)
	}
}

//RemoteHost struct
type RemoteHost struct {
	Network string
	Addr    string
	Config  ssh.ClientConfig
}

//NewRemoteHostTcp4 create new tcp4 host
func NewRemoteHostTcp4(addr string, config ssh.ClientConfig) (host RemoteHost, err error) {
	parsed, err := url.Parse(fmt.Sprintf("ssh://%s", addr))
	if err != nil {
		return
	}
	if !strings.Contains(parsed.Host, ":") {
		parsed.Host = parsed.Host + ":22"
	}
	if parsed.User != nil && parsed.User.Username() != "" {
		config.User = parsed.User.Username()
	} else {
		config.User, err = getCurrentUser()
		if err != nil {
			return
		}
	}
	host = RemoteHost{
		Network: "tcp4",
		Addr:    parsed.Host,
		Config:  config,
	}
	return
}

//RemoteRunner struct
type RemoteRunner struct {
	waitGroup *sync.WaitGroup
}

//NewRemoteRunner new RemoteRunner
func NewRemoteRunner() *RemoteRunner {
	return &RemoteRunner{}
}

//TailForever run tail on ssh server
func (rr *RemoteRunner) TailForever(file string, useSudo bool, hosts ...RemoteHost) (err error) {
	cmd := fmt.Sprintf("tail -f %s", file)
	if useSudo {
		cmd = "sudo " + cmd
	}
	return rr.runMulti(cmd, hosts...)
}

func (rr *RemoteRunner) messageReceive(endpoint string, data []byte) {
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fmt.Printf("%s: %s\n", endpoint, line)
	}
}

func (rr *RemoteRunner) run(cmd string, host RemoteHost) (err error) {
	defer func() {
		if err != nil {
			log.Printf("%s@%s: error: %s", host.Config.User, host.Addr, err)
		}
	}()
	// log.Printf("%s@%s: connecting", host.Config.User, host.Addr)
	client, err := ssh.Dial(host.Network, host.Addr, &host.Config)
	if err != nil {
		return err
	}
	defer client.Close()
	log.Printf("%s@%s: connected", host.Config.User, host.Addr)
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	endpointOut := fmt.Sprintf("%s@%s/stdout", host.Config.User, host.Addr)
	session.Stdout = &WriterCallback{Endpoint: endpointOut, Callback: rr.messageReceive}
	endpointErr := fmt.Sprintf("%s@%s/stderr", host.Config.User, host.Addr)
	session.Stderr = &WriterCallback{Endpoint: endpointErr, Callback: rr.messageReceive}
	log.Printf("%s@%s: run '%s'", host.Config.User, host.Addr, cmd)
	session.Run(cmd)
	return
}

//WriterCallbackFunc callback func for WriterCallback
type WriterCallbackFunc func(endpoint string, data []byte)

//WriterCallback io.Writer who call callback func
type WriterCallback struct {
	io.Writer
	Endpoint string
	Callback WriterCallbackFunc
}

func (wc *WriterCallback) Write(data []byte) (n int, err error) {
	wc.Callback(wc.Endpoint, data)
	return len(data), nil
}

func (rr *RemoteRunner) runMulti(cmd string, hosts ...RemoteHost) (err error) {
	rr.waitGroup = &sync.WaitGroup{}
	for _, host := range hosts {
		rr.waitGroup.Add(1)
		go func(host RemoteHost) {
			defer rr.waitGroup.Done()
			for {
				err = rr.run(cmd, host)
				if err == nil {
					break
				}
				time.Sleep(time.Second)
			}
		}(host)
	}
	rr.waitGroup.Wait()
	return
}

func getKeyFile(keyPath string) (key ssh.Signer, err error) {
	if keyPath == "" {
		keyPath = "~/.ssh/id_rsa"
	}
	if strings.HasPrefix(keyPath, "~/") {
		usr, err := user.Current()
		if err != nil {
			return nil, err
		}
		keyPath = usr.HomeDir + keyPath[1:]
	}
	log.Printf("load ssh key: %s", keyPath)
	buf, err := ioutil.ReadFile(keyPath)
	if err == nil {
		key, err = ssh.ParsePrivateKey(buf)
	}
	return
}

func getCurrentUser() (username string, err error) {
	usr, err := user.Current()
	if err != nil {
		username = usr.Username
	}
	return
}
