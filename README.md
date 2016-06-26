Octobus
=======

Fast remote command executor over SSH.

# Install

```
# go get github.com/miolini/octobus
```

# Usage:
```
  --cmd string
    	remote command (default "uname -a")
  --hosts string
    	hosts, support @filepath for loading hosts from file
  --key string
    	private key
  --pass string
    	optional user password
  --reconnect
    	reconnect on disconnected sessions (default false)
  --user string
    	remote user (default "root")
  --verbose
    	verbose mode (default false)
```