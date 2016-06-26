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
    	hosts
  --key string
    	private key
  --pass string
    	optional user password
  --user string
    	remote user (default "root")
  --verbose
    	verbose mode
```

# Issues

- do not support encrypted ssh keys
