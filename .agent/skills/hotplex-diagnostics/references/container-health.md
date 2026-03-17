# Container Health Reference

Comprehensive guide for diagnosing container health across any Docker deployment.

## Health Check Indicators

### Container States

| State | Description | Action |
|:------|:------------|:-------|
| `running` | Normal operation | None |
| `paused` | Frozen, memory retained | Unpause if intentional |
| `restarting` | Crash loop | Check logs |
| `exited` | Stopped normally | Start if needed |
| `dead` | Stopped abnormally | Investigate, recreate |
| `removing` | Being deleted | Wait |

### Health Status (Docker Healthcheck)

| Status | Meaning | Action |
|:-------|:--------|:-------|
| `healthy` | Passing health checks | Normal |
| `starting` | Initial grace period | Wait |
| `unhealthy` | Failing health checks | Investigate immediately |
| `n/a` | No healthcheck defined | Monitor manually |

## Resource Thresholds

### CPU Usage

| Level | Threshold | Investigation |
|:------|:----------|:--------------|
| Normal | < 50% | None |
| Elevated | 50-80% | Monitor trends |
| High | 80-100% | Check processes, scale if needed |
| Sustained High | >80% for >5min | Potential issue |

### Memory Usage

| Level | Threshold | Investigation |
|:------|:----------|:--------------|
| Normal | < 70% | None |
| Elevated | 70-85% | Monitor for leaks |
| High | 85-95% | Risk of OOM |
| Critical | > 95% | Immediate action needed |

### Network I/O

| Pattern | Indicator | Action |
|:---------|:----------|:-------|
| High TX | Sending lots of data | Check for data exfiltration, heavy API usage |
| High RX | Receiving lots of data | Check for DoS, heavy load |
| Unbalanced | TX >> RX or vice versa | May indicate issue |

## Common Issues by Container Type

### Database Containers

```bash
# Check connection limits
docker exec <container> mysql -e "SHOW PROCESSLIST;" 2>/dev/null
docker exec <container> psql -c "SELECT count(*) FROM pg_stat_activity;" 2>/dev/null

# Check disk usage
docker exec <container> du -sh /var/lib/mysql 2>/dev/null
docker exec <container> du -sh /var/lib/postgresql 2>/dev/null
```

### Web Server Containers

```bash
# Check worker processes
docker exec <container> ps aux | grep -E "nginx|apache|node" | wc -l

# Check response time
time docker exec <container> curl -s localhost/health > /dev/null
```

### Application Containers

```bash
# Check for zombie processes
docker exec <container> ps aux | awk '$8=="Z" {print}'

# Check open files
docker exec <container> lsof 2>/dev/null | wc -l
```

## Log Patterns to Watch

### Critical Patterns

```bash
# Out of memory
grep -i "out of memory\|oom\|cannot allocate"

# Disk full
grep -i "no space left\|disk full\|ENOSPC"

# Network failures
grep -i "connection refused\|network unreachable\|ECONNREFUSED"

# Permission issues
grep -i "permission denied\|EACCES"
```

### Warning Patterns

```bash
# High latency
grep -i "timeout\|slow\|latency"

# Retry loops
grep -i "retry\|retrying\|attempt.*failed"

# Resource warnings
grep -i "high cpu\|high memory\|resource"
```

## Container Restart Analysis

### Check Restart Policy

```bash
docker inspect <container> --format='{{.HostConfig.RestartPolicy.Name}}'
# on-failure = restart only on failure
# always = always restart
# unless-stopped = restart unless manually stopped
```

### Restart Count

```bash
docker inspect <container> --format='Restarts: {{.RestartCount}}, Started: {{.State.StartedAt}}'
```

### Frequent Restarts Investigation

```bash
# Get last 10 restart times
docker inspect <container> --format='{{range .State.Health.Log}} {{.End}} {{.ExitCode}} {{.Output}}{{end}}' | tail -10
```

## Network Diagnostics

### Container Network Inspection

```bash
# List networks
docker network ls

# Inspect specific network
docker network inspect <network-name>

# Check container's network
docker inspect <container> --format='{{range $k, $v := .NetworkSettings.Networks}}{{$k}} {{end}}'
```

### Inter-Container Connectivity

```bash
# Test from container A to container B
docker exec <container-a> ping -c 3 <container-b>

# Check DNS resolution
docker exec <container> nslookup <other-container>
```

## Volume/Disk Diagnostics

### Volume Usage

```bash
# List volumes
docker volume ls

# Check volume size
docker volume inspect <volume> --format='{{.Mountpoint}}'
docker run --rm -v <volume>:/data alpine du -sh /data

# Find large files
docker exec <container> find / -type f -size +100M 2>/dev/null
```

### Bind Mount Issues

```bash
# Check mount points
docker inspect <container> --format='{{range .Mounts}}Source: {{.Source}} -> Dest: {{.Destination}}{{println}}{{end}}'

# Verify host path exists
docker inspect <container> --format='{{range .Mounts}}{{.Source}}{{println}}{{end}}' | xargs ls -la
```
