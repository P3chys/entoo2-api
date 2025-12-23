# GitHub Actions Workflows

This directory contains GitHub Actions workflows for the Entoo2 API.

## Available Workflows

### 1. CI Workflow (`ci.yml`)

**Trigger:** Automatic on push/pull request to `main` branch

**Purpose:** Continuous Integration - runs tests, linting, and builds

**Jobs:**
- `test`: Runs Go tests with PostgreSQL service
- `lint`: Runs golangci-lint for code quality
- `build`: Builds the Go application
- `docker`: Builds Docker image (doesn't push)

### 2. Deploy Workflow (`deploy.yml`)

**Trigger:** Manual (workflow_dispatch)

**Purpose:** Builds Docker image and deploys to Hetzner server

**Jobs:**
- Build and push Docker image to GitHub Container Registry (GHCR)
- SSH into Hetzner server
- Pull latest image
- Restart backend service

## Setup Instructions

### Prerequisites

1. **GitHub Container Registry Access**
   - Go to Settings → Actions → General
   - Under "Workflow permissions", select "Read and write permissions"
   - Save changes

2. **Repository Secrets**

   Add these secrets in Settings → Secrets and variables → Actions:

   - `SSH_HOST`: Your Hetzner server IP address (e.g., `123.45.67.89`)
   - `SSH_USER`: SSH username (e.g., `root` or `deploy`)
   - `SSH_PRIVATE_KEY`: Private SSH key for authentication

### Generating SSH Key

```bash
# Generate a new SSH key pair
ssh-keygen -t ed25519 -C "github-actions-entoo2-api" -f ~/.ssh/github_deploy_api

# Display private key (copy to GitHub secret SSH_PRIVATE_KEY)
cat ~/.ssh/github_deploy_api

# Display public key (add to server's ~/.ssh/authorized_keys)
cat ~/.ssh/github_deploy_api.pub
```

### Server Setup

Ensure your Hetzner server has:

1. Docker and Docker Compose installed
2. Deployment directory at `/opt/entoo2`
3. `docker-compose.prod.yml` file
4. `.env.production` file with all required variables
5. Public SSH key added to `~/.ssh/authorized_keys`

## Using the Deploy Workflow

### Manual Deployment

1. Go to the repository on GitHub
2. Click "Actions" tab
3. Select "Deploy to Hetzner" workflow
4. Click "Run workflow" button
5. Select environment (production/staging)
6. Click "Run workflow"

### What Happens During Deployment

1. **Build Phase**
   - Checks out code
   - Sets up Docker Buildx
   - Logs into GitHub Container Registry
   - Builds Docker image with metadata tags
   - Pushes image to GHCR with tags:
     - `latest` (for main branch)
     - Branch name
     - Commit SHA

2. **Deploy Phase**
   - Connects to server via SSH
   - Navigates to `/opt/entoo2`
   - Pulls latest backend image
   - Restarts backend service with zero-downtime
   - Verifies service is running

3. **Cleanup Phase**
   - Removes SSH key from runner
   - Notifies deployment status

## Docker Image Tags

Images are tagged with multiple tags for flexibility:

- `ghcr.io/USERNAME/entoo2-api:latest` - Latest main branch build
- `ghcr.io/USERNAME/entoo2-api:main` - Main branch
- `ghcr.io/USERNAME/entoo2-api:main-abc123` - Commit SHA
- `ghcr.io/USERNAME/entoo2-api:v1.0.0` - Semantic version (if tagged)

## Troubleshooting

### Deployment Fails at SSH Step

**Issue:** Cannot connect to server

**Solutions:**
- Verify `SSH_HOST` secret is correct
- Verify `SSH_USER` secret is correct
- Verify `SSH_PRIVATE_KEY` is the full private key including headers
- Test SSH connection manually: `ssh -i ~/.ssh/github_deploy_api user@host`
- Check server firewall allows SSH (port 22)

### Image Pull Fails on Server

**Issue:** Cannot pull image from GHCR

**Solutions:**
- Ensure package is public, or login to GHCR on server:
  ```bash
  echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin
  ```
- Verify image exists: Check GitHub → Packages
- Check network connectivity on server

### Service Won't Start

**Issue:** Docker container fails to start

**Solutions:**
- Check logs: `docker compose -f docker-compose.prod.yml logs backend`
- Verify environment variables in `.env.production`
- Ensure database is running: `docker compose ps postgres`
- Check disk space: `df -h`

### Permission Denied

**Issue:** SSH user cannot run Docker commands

**Solutions:**
- Add user to docker group: `usermod -aG docker username`
- Logout and login again, or run: `newgrp docker`
- Verify: `docker ps` should work without sudo

## Security Best Practices

1. **SSH Keys**
   - Use separate SSH keys for GitHub Actions
   - Use Ed25519 keys (more secure, smaller)
   - Rotate keys periodically

2. **Secrets Management**
   - Never commit secrets to repository
   - Use GitHub Secrets for sensitive data
   - Limit secret access to necessary workflows

3. **Docker Images**
   - Keep images private if containing sensitive code
   - Scan images for vulnerabilities
   - Use specific tags instead of `latest` in production

4. **Server Access**
   - Limit SSH access to specific IPs if possible
   - Use fail2ban to prevent brute force
   - Keep server and Docker updated

## Monitoring Deployments

### View Deployment Logs

- Go to Actions → Select workflow run
- Click on job to see detailed logs
- Check "Deploy to Hetzner" step for SSH output

### Verify Deployment on Server

```bash
# SSH into server
ssh user@SERVER_IP

# Check running containers
docker ps

# View backend logs
docker logs entoo2-backend -f

# Check service health
docker compose -f docker-compose.prod.yml ps backend
```

## Rolling Back

If a deployment causes issues:

```bash
# SSH into server
ssh user@SERVER_IP

cd /opt/entoo2

# List available images
docker images | grep entoo2-api

# Update .env.production to use previous tag
nano .env.production
# Change IMAGE_TAG to previous version

# Restart with previous image
docker compose -f docker-compose.prod.yml up -d backend
```

## Additional Resources

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Docker Documentation](https://docs.docker.com/)
- [GHCR Documentation](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
- [Main Deployment Guide](../../../DEPLOYMENT.md)
