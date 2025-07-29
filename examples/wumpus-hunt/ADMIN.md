# Wumpus Hunt - Administrator Guide

This guide provides comprehensive information for setting up, configuring, and maintaining a Wumpus Hunt MUD server.

## Table of Contents

1. [Installation and Setup](#installation-and-setup)
2. [Server Configuration](#server-configuration)
3. [Monitoring and Metrics](#monitoring-and-metrics)
4. [Performance Tuning](#performance-tuning)
5. [Backup and Recovery](#backup-and-recovery)
6. [Troubleshooting](#troubleshooting)
7. [Security Considerations](#security-considerations)

## Installation and Setup

### Prerequisites

- **Go 1.21+**: Required for building and running the server
- **System Resources**: Minimum 512MB RAM, 1GB recommended for 100+ concurrent players
- **Network**: TCP port for game connections, optional WebSocket port for web clients
- **Storage**: File system access for persistent player data

### Basic Installation

```bash
# Clone the MuddyCore repository
git clone https://github.com/catalystcommunity/muddycore.git
cd muddycore/examples/wumpus-hunt

# Build the application
go build -o wumpus-hunt .

# Test the build
./wumpus-hunt --help
```

### Systemd Service Setup (Linux)

Create `/etc/systemd/system/wumpus-hunt.service`:

```ini
[Unit]
Description=Wumpus Hunt MUD Server
After=network.target

[Service]
Type=simple
User=mudadmin
Group=mudadmin
WorkingDirectory=/opt/wumpus-hunt
ExecStart=/opt/wumpus-hunt/wumpus-hunt server --host 0.0.0.0 --port 8080 --log-level info
Restart=always
RestartSec=10

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/wumpus-hunt/portal/data /opt/wumpus-hunt/instance/data

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable wumpus-hunt
sudo systemctl start wumpus-hunt
sudo systemctl status wumpus-hunt
```

## Server Configuration

### Command Line Options

```bash
# Basic server configuration
./wumpus-hunt server [OPTIONS]

Options:
  --host HOST              Server bind address (default: "localhost")
  --port PORT              TCP port for game connections (default: "8080")
  --enable-ws              Enable WebSocket support (default: false)
  --ws-port PORT           WebSocket port (default: "8081")
  --max-connections NUM    Maximum concurrent connections (default: 100)
  --log-level LEVEL        Logging level: debug, info, warn, error (default: "info")
  --enable-monitoring      Enable metrics and monitoring (default: false)
  --metrics-port PORT      Metrics HTTP server port (default: "9090")
```

### Environment Variables

Alternative configuration via environment variables:

```bash
export WUMPUS_HOST="0.0.0.0"
export WUMPUS_PORT="8080"
export WUMPUS_WS_PORT="8081"
export WUMPUS_ENABLE_WS="true"
export WUMPUS_MAX_CONNECTIONS="500"
export WUMPUS_LOG_LEVEL="info"
export WUMPUS_ENABLE_MONITORING="true"
export WUMPUS_METRICS_PORT="9090"
```

### Production Configuration Example

```bash
#!/bin/bash
# production-start.sh
./wumpus-hunt server \
  --host 0.0.0.0 \
  --port 8080 \
  --enable-ws \
  --ws-port 8081 \
  --max-connections 500 \
  --log-level info \
  --enable-monitoring \
  --metrics-port 9090 \
  2>&1 | tee -a logs/wumpus-hunt.log
```

## Monitoring and Metrics

### Built-in Monitoring Dashboard

When monitoring is enabled, access the dashboard at:
- **URL**: `http://your-server:9090/dashboard`
- **API**: `http://your-server:9090/api/metrics`

### Key Metrics to Monitor

#### Player Metrics
```json
{
  "active_players": 45,
  "total_registered": 1250,
  "concurrent_connections": 47,
  "average_session_duration": "25m30s"
}
```

#### Game Instance Metrics
```json
{
  "active_instances": 12,
  "instances_created_today": 89,
  "cleanup_operations": 156,
  "average_instance_lifetime": "15m45s"
}
```

#### Performance Metrics
```json
{
  "properties_cache_hit_rate": 0.95,
  "properties_access_per_second": 450,
  "memory_usage_mb": 128,
  "cpu_usage_percent": 15.2
}
```

#### Error Metrics
```json
{
  "authentication_failures": 3,
  "properties_corruption_recoveries": 1,
  "lua_script_errors": 0,
  "network_disconnections": 12
}
```

### Custom Monitoring Integration

The server exposes Prometheus-compatible metrics at `/metrics`:

```bash
# Scrape metrics with curl
curl http://localhost:9090/metrics

# Example Prometheus configuration
scrape_configs:
  - job_name: 'wumpus-hunt'
    static_configs:
      - targets: ['your-server:9090']
```

### Log Analysis

Log files are structured JSON for easy parsing:

```bash
# Find authentication issues
grep "authentication_failure" logs/wumpus-hunt.log | jq '.'

# Monitor instance creation
grep "instance_created" logs/wumpus-hunt.log | jq '.instance_id, .player_id'

# Track performance issues
grep "properties_cache_miss" logs/wumpus-hunt.log | jq '.key, .access_time'
```

## Performance Tuning

### Memory Optimization

#### Property Cache Configuration
```go
// Default cache settings (can be tuned in code)
maxSize := 10000      // Maximum cached properties
ttl := 5 * time.Minute // Cache expiration time
```

For high-traffic servers, consider:
- Increasing cache size to 50,000+ entries
- Reducing TTL to 2-3 minutes for more frequent updates
- Monitor cache hit rates - aim for >90%

#### Connection Pooling
```bash
# Adjust max connections based on server capacity
# Rule of thumb: 100-200 connections per 1GB RAM
--max-connections 200  # For 1GB RAM server
--max-connections 500  # For 2GB+ RAM server
```

### Storage Optimization

#### Cleanup Configuration
```go
// Adjust cleanup intervals for your server load
cleanupInterval := 1 * time.Minute  // Default
maxOrphanedAge := 30 * time.Minute  // Default
```

#### File System Recommendations
- Use SSD storage for `portal/data/` and `instance/data/` directories
- Separate data directories on different mount points if possible
- Regular disk space monitoring - plan for 1-5MB per active player

### Network Optimization

#### TCP Settings
```bash
# Linux system tuning for high connection counts
echo 'net.core.somaxconn = 1024' >> /etc/sysctl.conf
echo 'net.ipv4.tcp_max_syn_backlog = 2048' >> /etc/sysctl.conf
sysctl -p
```

#### WebSocket Configuration
- Enable WebSocket compression for better bandwidth usage
- Consider load balancing for multiple server instances
- Monitor WebSocket connection stability

## Backup and Recovery

### Automated Backup Script

```bash
#!/bin/bash
# backup-wumpus.sh
BACKUP_DIR="/backups/wumpus-hunt"
DATE=$(date +%Y%m%d_%H%M%S)
DATA_DIR="/opt/wumpus-hunt"

# Create backup directory
mkdir -p "$BACKUP_DIR/$DATE"

# Backup player data
cp -r "$DATA_DIR/portal/data/" "$BACKUP_DIR/$DATE/portal_data/"
cp -r "$DATA_DIR/instance/data/" "$BACKUP_DIR/$DATE/instance_data/"

# Backup configuration
cp "$DATA_DIR/wumpus-hunt" "$BACKUP_DIR/$DATE/"

# Compress backup
tar -czf "$BACKUP_DIR/wumpus-hunt-$DATE.tar.gz" -C "$BACKUP_DIR" "$DATE"
rm -rf "$BACKUP_DIR/$DATE"

# Cleanup old backups (keep 30 days)
find "$BACKUP_DIR" -name "*.tar.gz" -mtime +30 -delete

echo "Backup completed: wumpus-hunt-$DATE.tar.gz"
```

### Recovery Procedures

#### Player Data Recovery
```bash
# Stop server
systemctl stop wumpus-hunt

# Restore from backup
tar -xzf /backups/wumpus-hunt/wumpus-hunt-20240125_120000.tar.gz
cp -r 20240125_120000/portal_data/* /opt/wumpus-hunt/portal/data/
cp -r 20240125_120000/instance_data/* /opt/wumpus-hunt/instance/data/

# Fix permissions
chown -R mudadmin:mudadmin /opt/wumpus-hunt/portal/data
chown -R mudadmin:mudadmin /opt/wumpus-hunt/instance/data

# Start server
systemctl start wumpus-hunt
```

#### Properties Corruption Recovery
The system includes automatic recovery mechanisms, but manual intervention may be needed:

```bash
# Check for corrupted player files
go run . admin check-players

# Repair specific player (example)
go run . admin repair-player --player-id="abc123" --backup-date="20240125"

# Bulk repair (emergency)
go run . admin repair-all --dry-run  # Test first
go run . admin repair-all            # Execute repairs
```

## Troubleshooting

### Common Issues

#### High Memory Usage
**Symptoms**: Server consuming excessive RAM, sluggish performance
**Solutions**:
- Reduce property cache size in configuration
- Implement more aggressive cleanup intervals
- Check for memory leaks in custom Lua scripts
- Monitor for orphaned game instances

#### Connection Issues
**Symptoms**: Players unable to connect, connection drops
**Solutions**:
- Check firewall settings for configured ports
- Verify network interface binding (0.0.0.0 vs localhost)
- Monitor connection limits and increase if needed
- Check system file descriptor limits

#### Properties Corruption
**Symptoms**: Players losing progress, authentication failures
**Solutions**:
- Enable automatic recovery mechanisms
- Restore from recent backup
- Use admin tools to repair specific players
- Implement more frequent backup schedules

#### Performance Degradation
**Symptoms**: Slow response times, high CPU usage
**Solutions**:
- Monitor property cache hit rates
- Optimize Lua scripts for efficiency
- Check for infinite loops in game logic
- Scale instance cleanup frequency

### Debug Mode

Run server in debug mode for detailed logging:

```bash
./wumpus-hunt server --log-level debug --enable-monitoring
```

Debug logs include:
- Property access patterns
- Cache hit/miss statistics
- Game state transitions
- Network connection details
- Lua script execution times

### Health Checks

```bash
# Basic connectivity test
nc -zv localhost 8080

# WebSocket test
curl -H "Upgrade: websocket" \
     -H "Connection: Upgrade" \
     -H "Sec-WebSocket-Key: test" \
     -H "Sec-WebSocket-Version: 13" \
     http://localhost:8081/

# Metrics endpoint test
curl http://localhost:9090/api/health
```

## Security Considerations

### Authentication Security
- Passwords are hashed using bcrypt with individual salts
- No plaintext password storage
- Failed authentication attempts are logged
- Consider implementing rate limiting for login attempts

### Network Security
```bash
# Firewall configuration example (iptables)
iptables -A INPUT -p tcp --dport 8080 -j ACCEPT  # Game port
iptables -A INPUT -p tcp --dport 8081 -j ACCEPT  # WebSocket port
iptables -A INPUT -p tcp --dport 9090 -s ADMIN_IP -j ACCEPT  # Metrics (admin only)
```

### File System Security
```bash
# Secure data directories
chmod 700 /opt/wumpus-hunt/portal/data
chmod 700 /opt/wumpus-hunt/instance/data
chown -R mudadmin:mudadmin /opt/wumpus-hunt
```

### Lua Script Security
- Scripts run in sandboxed environment
- Dangerous functions (io, os) are disabled
- Resource limits prevent infinite loops
- Script validation prevents code injection

### Data Privacy
- Player passwords are properly hashed
- Game statistics stored in Properties are not exposed
- Admin access to player data should be logged
- Consider GDPR compliance for player data retention

## Maintenance Schedule

### Daily Tasks
- Monitor server status and performance metrics
- Check log files for errors or warnings
- Verify backup completion
- Review active player counts and connection stability

### Weekly Tasks
- Analyze performance trends and resource usage
- Review and rotate log files
- Test backup recovery procedures
- Update monitoring dashboard configurations

### Monthly Tasks
- Full system backup verification
- Performance optimization review
- Security audit of access logs
- Capacity planning based on growth trends

### Quarterly Tasks
- Major backup testing and restoration procedures
- Security updates and dependency reviews
- Performance benchmark comparisons
- Documentation updates and admin training

For additional support or advanced configuration options, refer to the [DESIGN.md](DESIGN.md) technical documentation or the MuddyCore framework documentation.