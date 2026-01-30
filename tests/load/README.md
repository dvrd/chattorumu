# Load Testing Guide

Complete load testing suite for the Jobsity Chat application using Grafana k6.

## 📋 Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [Test Scenarios](#test-scenarios)
- [Running Tests](#running-tests)
- [Analyzing Results](#analyzing-results)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## 🎯 Overview

This load testing suite simulates thousands of concurrent users to test the resilience, performance, and scalability of the chat application.

**What we test:**
- WebSocket connection handling (up to 1000+ concurrent)
- Message throughput and latency
- Stock bot command processing via RabbitMQ
- Database connection pool behavior
- Memory leaks and resource exhaustion
- System recovery after traffic spikes
- Multi-room broadcast efficiency

**Tools used:**
- **k6** - Load testing tool by Grafana
- **Prometheus** - Metrics collection (already configured)
- **Grafana** - Real-time monitoring and dashboards

## 🚀 Quick Start

### 1. Setup Environment

```bash
# Start all services (chat-server, postgres, rabbitmq, prometheus, grafana)
task load:setup

# Wait for all services to be healthy (~10 seconds)
```

### 2. Run Your First Test

```bash
# Quick realistic test (~13 minutes)
task load:run:chat
```

### 3. View Results

Open Grafana dashboard:
```
http://localhost:3100
```

Your app metrics are already there. For k6 metrics:
- Import dashboard ID: **18595** (k6 Load Testing Results - Prometheus)

## 📊 Test Scenarios

### Scenario 1: Basic Connection Load (Baseline)
**File:** `scenarios/01-connection-load.js`
**Duration:** 17 minutes
**Load Pattern:** 0 → 1000 → 0 users

Tests maximum concurrent WebSocket connections. Establishes baseline for:
- Connection establishment time
- Memory usage per connection
- Goroutine count at scale
- System stability under load

**When to run:** First test to establish capacity limits.

```bash
task load:run:connection
```

**Success criteria:**
- ✅ Connection success rate > 99%
- ✅ P95 connection time < 1 second
- ✅ No memory leaks
- ✅ Stable goroutine count

---

### Scenario 2: Active Chat Load (Realistic)
**File:** `scenarios/02-active-chat-load.js`
**Duration:** 13 minutes
**Load Pattern:** 500 users across 10 chatrooms

Simulates real user behavior with varied activity levels. Each user:
- Sends 1-5 messages per minute
- 20% send stock commands
- Distributed across multiple chatrooms

**When to run:** Most important test for production readiness.

```bash
task load:run:chat
```

**Success criteria:**
- ✅ Message delivery rate > 99.9%
- ✅ P95 message latency < 200ms
- ✅ Stock command latency < 2s (P95)
- ✅ No RabbitMQ queue backup

---

### Scenario 3: Stock Bot Stress Test
**File:** `scenarios/03-stock-bot-stress.js`
**Duration:** 6.5 minutes
**Load Pattern:** 100 users, 1 command every 10 seconds

Stress tests the stock bot subsystem:
- ~600 commands per minute (~10/second)
- Tests RabbitMQ throughput
- Validates Stooq API integration
- Checks database write performance

**When to run:** When stock bot is critical path.

```bash
task load:run:stock
```

**Success criteria:**
- ✅ Command processing < 2s (P95)
- ✅ All commands get responses
- ✅ RabbitMQ not dropping messages
- ✅ Stock bot stable (no crashes)

---

### Scenario 4: Spike Test (Resilience)
**File:** `scenarios/04-spike-test.js`
**Duration:** 10 minutes
**Load Pattern:** 100 → **1000** → 100 users (30s spike)

Tests system behavior during sudden traffic surges:
- Rapid connection spike
- System recovery
- Error handling under pressure
- Resource cleanup

**When to run:** Before production deployment, after code changes.

```bash
task load:run:spike
```

**Success criteria:**
- ✅ No crashes during spike
- ✅ Connection success rate > 95% during spike
- ✅ System recovers to baseline
- ✅ Error rate < 1%

---

### Scenario 5: Soak Test (Stability)
**File:** `scenarios/05-soak-test.js`
**Duration:** 2+ hours (or 15 minutes with SHORT_SOAK)
**Load Pattern:** 300 users continuous

Long-running test to detect:
- Memory leaks
- Goroutine leaks
- Database connection leaks
- Performance degradation over time

**When to run:** Before major releases, overnight.

```bash
# Full soak test (2+ hours)
task load:run:soak

# Quick validation (15 minutes)
task load:run:soak-short
```

**Success criteria:**
- ✅ Memory usage stable (no continuous growth)
- ✅ Goroutine count stable
- ✅ Performance consistent throughout
- ✅ No resource exhaustion

---

### Scenario 6: Multi-Room Load Distribution
**File:** `scenarios/06-multi-room-load.js`
**Duration:** 15 minutes
**Load Pattern:** 1000 users across 50 chatrooms (Pareto distribution)

Tests hub broadcast efficiency with uneven distribution:
- Popular rooms: 80% of users
- Quiet rooms: 20% of users
- Tests broadcast isolation
- Validates fair message delivery

**When to run:** When broadcast logic changes.

```bash
task load:run:multiroom
```

**Success criteria:**
- ✅ Large rooms don't block small rooms
- ✅ Consistent latency across room sizes
- ✅ No broadcast channel overflow
- ✅ Fair message delivery

---

## 🏃 Running Tests

### Individual Scenarios

```bash
# Run specific test
task load:run:connection
task load:run:chat
task load:run:stock
task load:run:spike
task load:run:soak
task load:run:multiroom
```

### All Scenarios

```bash
# Run all tests sequentially (~1 hour, excludes full soak)
task load:run:all
```

### Custom k6 Run

```bash
# Run with custom options
docker-compose -f containers/docker-compose.yml --profile load-testing run --rm \
  -e BASE_URL=http://chat-server:8080 \
  k6 run --vus 100 --duration 5m /scripts/scenarios/02-active-chat-load.js
```

## 📈 Analyzing Results

### 1. Grafana Dashboards

**Your App Metrics** (already configured):
```
http://localhost:3100/d/<your-dashboard-uid>/jobsity-chat-monitoring
```

Key panels to watch:
- **WebSocket Connections Active** - Should match user count
- **Messages per Minute** - Throughput indicator
- **P95 Latency** - Performance indicator
- **Memory Usage** - Leak detection
- **Goroutines** - Goroutine leak detection
- **DB Connection Pool** - Resource usage

**k6 Load Test Metrics:**
```
Import Dashboard ID: 18595
```

Panels include:
- Virtual Users (VUs) over time
- Request rate
- Error rate
- Response time percentiles
- Custom metrics (message latency, stock commands)

### 2. Prometheus Queries

Access Prometheus directly:
```
http://localhost:9090
```

**Useful queries:**

```promql
# Connection count
websocket_connections_active

# Message rate
rate(websocket_messages_sent_total[1m])

# Message latency (from k6)
histogram_quantile(0.95, rate(chat_message_latency_ms_bucket[5m]))

# Stock command processing time
histogram_quantile(0.95, rate(stock_command_latency_ms_bucket[5m]))

# Error rate
rate(websocket_errors[5m])

# Memory usage
go_memstats_heap_inuse_bytes / 1024 / 1024

# Goroutine count
go_goroutines

# DB connections in use
db_connections_in_use
```

### 3. k6 Terminal Output

k6 prints a summary at the end of each test:

```
✓ websocket connected successfully
✓ message delivery success

checks.........................: 99.85% ✓ 1997  ✗ 3
chat_messages_received.........: 45623  381/s
chat_messages_sent.............: 45680  381/s
chat_message_latency_ms........: avg=125  min=45  med=118  max=892  p(95)=198
websocket_connection_success...: 99.90% ✓ 1998  ✗ 2
websocket_errors...............: 2      0.017/s
ws_connecting..................: avg=245ms min=112ms med=234ms max=567ms p(95)=342ms
```

**What to look for:**
- ✅ High check pass rate (>95%)
- ✅ Low error count
- ✅ Latency within thresholds
- ✅ No failed thresholds

### 4. Identifying Bottlenecks

| Symptom | Likely Cause | Fix |
|---------|--------------|-----|
| High connection time | OS file descriptor limits | Increase ulimit |
| Message delivery lag | Broadcast channel full | Increase buffer size |
| High DB query time | Connection pool exhausted | Increase max connections |
| Stock command delays | RabbitMQ backup | Scale stock bot horizontally |
| Memory continuously growing | Memory leak | Profile with pprof |
| High CPU usage | JSON marshaling overhead | Use binary protocol |

## 🎯 Best Practices

### Before Testing

1. **Baseline Performance**
   ```bash
   # Record current metrics
   curl http://localhost:9090/api/v1/query?query=go_memstats_heap_inuse_bytes > baseline.txt
   ```

2. **Increase System Limits**
   ```bash
   # Increase file descriptors
   ulimit -n 65536

   # In docker-compose.yml (already configured):
   ulimits:
     nofile:
       soft: 65536
       hard: 65536
   ```

3. **Pre-create Test Data**
   - Chatrooms created automatically by tests
   - Database should be in clean state

### During Testing

1. **Monitor in Real-Time**
   - Keep Grafana dashboard open
   - Watch for anomalies
   - Check docker logs if errors occur

2. **Don't Test in Production**
   - Always use isolated environment
   - Never point tests at production URLs

3. **Cool Down Between Tests**
   ```bash
   # Allow system to stabilize
   sleep 30
   ```

### After Testing

1. **Document Results**
   - Screenshot Grafana dashboards
   - Save k6 summary output
   - Note any anomalies

2. **Compare with Baseline**
   - Memory usage before/after
   - Performance metrics
   - Identify regressions

3. **Clean Up**
   ```bash
   task load:clean
   ```

## 🔧 Troubleshooting

### Test Won't Start

**Error:** `Cannot connect to chat-server:8080`

```bash
# Check if services are running
docker ps

# Restart services
task load:setup

# Check chat-server logs
docker logs jobsity-chat-server
```

### High Error Rate

**Error:** `connection refused` or `timeout`

```bash
# Check system resources
docker stats

# Increase timeouts in scenarios/*.js
# Or reduce user count in options.stages
```

### Prometheus Not Receiving Metrics

**Error:** k6 metrics not in Prometheus

```bash
# Verify Prometheus is accepting remote write
curl http://localhost:9090/api/v1/write

# Check k6 environment variables in docker-compose.yml
K6_PROMETHEUS_RW_SERVER_URL=http://prometheus:9090/api/v1/write
```

### Memory Leak Detected

**Symptom:** Memory continuously growing

```bash
# Profile heap during test
curl http://localhost:8080/debug/pprof/heap > heap.prof

# Analyze
go tool pprof heap.prof

# Check goroutines
curl http://localhost:8080/debug/pprof/goroutine?debug=1
```

### Tests Running Slow

**Symptom:** Long connection times, high latency

```bash
# Check system resources
docker stats

# Reduce concurrent users
# Modify options.stages in scenario files

# Check for rate limiting
docker logs jobsity-chat-server | grep -i "rate limit"
```

## 📚 Additional Resources

- [k6 Documentation](https://k6.io/docs/)
- [k6 WebSocket Guide](https://k6.io/docs/using-k6/protocols/websockets/)
- [Grafana k6 Integration](https://k6.io/docs/results-output/real-time/prometheus-remote-write/)
- [Load Testing Best Practices](https://k6.io/docs/testing-guides/api-load-testing/)

## 🎓 Understanding Load Testing Metrics

### Response Time Targets

| Percentile | Excellent | Good | Acceptable | Poor |
|------------|-----------|------|------------|------|
| P50 (median) | < 50ms | < 100ms | < 200ms | > 200ms |
| P95 | < 100ms | < 200ms | < 500ms | > 500ms |
| P99 | < 200ms | < 500ms | < 1000ms | > 1000ms |

### Capacity Planning

```
Max Capacity = min(
  DB_CONNECTIONS / QUERIES_PER_REQUEST,
  NETWORK_BANDWIDTH / MESSAGE_SIZE,
  MEMORY_AVAILABLE / BYTES_PER_CONNECTION,
  CPU_CORES * GOROUTINES_PER_CORE
)

For this application:
- Memory per connection: ~20KB
- 8GB server → ~400k theoretical max
- Practical max: 40-80k per instance (10-20%)
```

### Error Budgets

| Metric | Target | Error Budget |
|--------|--------|--------------|
| Connection Success | 99.9% | 0.1% (1 in 1000) |
| Message Delivery | 99.9% | 0.1% (1 in 1000) |
| Stock Commands | 95% | 5% (API dependency) |
| Uptime | 99.9% | 8.76 hours/year |

---

## 📝 Summary

**Quick Commands:**
```bash
task load:setup           # Setup environment
task load:run:chat        # Quick realistic test (13 min)
task load:run:spike       # Test resilience (10 min)
task load:run:soak-short  # Memory leak check (15 min)
task load:run:all         # All tests (~1 hour)
task load:clean           # Cleanup
```

**What Success Looks Like:**
- ✅ All tests pass thresholds
- ✅ No memory leaks in soak test
- ✅ System recovers after spike
- ✅ Consistent performance across scenarios
- ✅ Error rate < 1%

**Next Steps After Testing:**
1. Document capacity limits
2. Set up alerting based on findings
3. Plan horizontal scaling strategy
4. Integrate tests into CI/CD
5. Run soak test weekly

---

**Need help?** Check the troubleshooting section or open an issue.

**Questions?** Review the k6 documentation or Grafana dashboards.

Happy load testing! 🚀
