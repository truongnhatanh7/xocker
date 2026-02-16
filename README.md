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

# Build
make build

# Run a command to change hostname in isolated container
sudo ./bin/xocker run --rootfs="./rootfs" -- /bin/sh -c "hostname iso-truongnhatanh7; hostname" 
> iso-truongnhatanh7

# Then on host computer, rerun hostname to check isolation
hostname
> host-truongnhatanh7

# Interactive mode (-i flag)
# Works by disable terminal canonical mode
#   and Set up PTY, master on Go runtime, slave connect to container
sudo ./bin/xocker run --rootfs="./rootfs" --level="dev" -i -- /bin/ash
> <now you're inside the container, try to echo something :D>
```

