# Re-Build Docker from Scratch - Xocker

This project is for learning Docker in depth

## Phase 1: Basic runtime

Implement container runtime with namespace isolation.
Concepts:
- Linux namespaces
- Filesystem isolation

After phase 1 it should be able to run on Linux:
```bash
# Create Alpine rootfs
mkdir /tmp/alpine-rootfs
docker export $(docker create alpine) | tar -C /tmp/alpine-rootfs -xf -

# Run a command
sudo ./build/hocker run --rootfs /tmp/alpine-rootfs /bin/echo "Hello from Hocker!"

# Interactive shell
sudo ./build/hocker run -it --rootfs /tmp/alpine-rootfs /bin/sh
```
Q: For Mac, we should run this in VM?