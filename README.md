# Re-Build Docker from Scratch - Xocker

This project is for learning Docker in depth

## Phase 1: Basic runtime

Implement container runtime with namespace isolation.
Concepts:
- Linux namespaces
- Filesystem isolation

After phase 1 it should be able to run on Linux:
```bash
# Get alpine rootfs
curl -LO https://dl-cdn.alpinelinux.org/alpine/v3.19/releases/x86_64/alpine-minirootfs-3.19.0-x86_64.tar.gz
tar -xzf alpine-minirootfs-3.19.0-x86_64.tar.gz -C rootfs

# Run a command (rootfs is hard-coded)
sudo env "PATH=$PATH" go run main.go run /bin/sh "echo 1"                                                                                   ❮❮❮
```

