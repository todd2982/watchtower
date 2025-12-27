<div align="center">
  <img src="./logo.png" width="450" />
  
  # Watchtower
  
  A process for automating Docker container base image updates.
  <br/><br/>
  
  [![GoDoc](https://godoc.org/github.com/todd2982/watchtower?status.svg)](https://godoc.org/github.com/todd2982/watchtower)
  [![Go Report Card](https://goreportcard.com/badge/github.com/todd2982/watchtower)](https://goreportcard.com/report/github.com/todd2982/watchtower)
  [![latest version](https://img.shields.io/github/tag/todd2982/watchtower.svg)](https://github.com/todd2982/watchtower/releases)
  [![Apache-2.0 License](https://img.shields.io/github/license/todd2982/watchtower.svg)](https://www.apache.org/licenses/LICENSE-2.0)
  [![Codacy Badge](https://app.codacy.com/project/badge/Grade/1c48cfb7646d4009aa8c6f71287670b8)](https://www.codacy.com/gh/todd2982/watchtower/dashboard?utm_source=github.com&amp;utm_medium=referral&amp;utm_content=todd2982/watchtower&amp;utm_campaign=Badge_Grade)
  [![Pulls from DockerHub](https://img.shields.io/docker/pulls/todd2982/watchtower.svg)](https://hub.docker.com/r/todd2982/watchtower)

</div>

## Quick Start

With watchtower you can update the running version of your containerized app simply by pushing a new image to the Docker Hub or your own image registry. 

Watchtower will pull down your new image, gracefully shut down your existing container and restart it with the same options that were used when it was deployed initially. Run the watchtower container with the following command:

```
$ docker run --detach \
    --name watchtower \
    --volume /var/run/docker.sock:/var/run/docker.sock \
    todd2982/watchtower
```

Watchtower is intended to be used in homelabs, media centers, local dev environments, and similar. We do **not** recommend using Watchtower in a commercial or production environment. If that is you, you should be looking into using Kubernetes. If that feels like too big a step for you, please look into solutions like [MicroK8s](https://microk8s.io/) and [k3s](https://k3s.io/) that take away a lot of the toil of running a Kubernetes cluster. 

## Security Considerations

⚠️ **Watchtower is designed for homelabs and local development, not production environments.** Please review these security considerations before deploying:

### Docker Socket Access

Watchtower requires access to `/var/run/docker.sock`, which grants **full control over all containers** on the host. This is equivalent to root access. Only run watchtower in trusted environments.

### HTTP API

The HTTP API (`--http-api-update`) exposes container update controls:

- **No TLS by default**: API requests are sent over unencrypted HTTP
- **Token authentication**: Use a strong token (see `--http-api-token` flag help)
- **Network exposure**: Bind to localhost only in untrusted networks using Docker port mapping: `-p 127.0.0.1:8080:8080`
- **Recommendation**: Use a reverse proxy with HTTPS for any non-local access

### Lifecycle Hooks

Lifecycle hooks (`--enable-lifecycle-hooks`) execute arbitrary commands inside containers:

- **Command injection risk**: Hooks run with container permissions
- **User-supplied commands**: Never use untrusted input in hook commands
- **Recommendation**: Carefully audit all lifecycle hook configurations

### Registry Credentials

- Watchtower may handle registry credentials for pulling images
- Credentials are passed to the Docker daemon and may appear in logs at TRACE level
- Store credentials securely and use registry access tokens when possible

### Reporting Security Issues

To report security vulnerabilities, please see our [Security Policy](SECURITY.md).

This project follows the [all-contributors](https://github.com/all-contributors/all-contributors) specification. Contributions of any kind welcome!
