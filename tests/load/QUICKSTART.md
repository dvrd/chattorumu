# Load Testing Quick Start

Get started with load testing in 5 minutes.

## Prerequisites

- Docker and Docker Compose installed
- Task (taskfile) installed
- All services running

## Step 1: Start Services (1 minute)

```bash
task load:setup
```

Wait for:
```
✅ Load testing environment ready!
```

## Step 2: Setup Grafana Dashboard (1 minute)

```bash
cd tests/load
./setup-grafana-k6-dashboard.sh
```

This imports the k6 dashboard into Grafana automatically.

## Step 3: Run Your First Test (13 minutes)

```bash
task load:run:chat
```

This runs the most realistic test scenario:
- 500 concurrent users
- Across 10 chatrooms
- Mix of chat messages and stock commands

## Step 4: Watch Results (real-time)

Open Grafana:
```
http://localhost:3100
```

**Your App Dashboard:**
- Already configured
- Shows WebSocket connections, message rates, latency, etc.

**k6 Dashboard:**
- Click "Dashboards" → "Browse"
- Select "k6 Load Testing Results"
- Watch metrics update in real-time

## Understanding the Results

### Terminal Output

At the end, you'll see:

```
✓ websocket connected successfully
✓ message delivery success

checks.........................: 99.85% ✓ 1997  ✗ 3
chat_messages_sent.............: 45680  381/s
chat_message_latency_ms........: p(95)=198ms
```

**Good signs:**
- ✅ Check rate > 95%
- ✅ P95 latency < 200ms
- ✅ Few errors (< 1%)

**Warning signs:**
- ⚠️ Check rate < 90%
- ⚠️ High latency (> 500ms)
- ⚠️ Many errors

### Grafana Dashboard

**Key metrics to watch:**

1. **WebSocket Connections Active**
   - Should reach 500 during test
   - Should drop to 0 at end

2. **Messages per Minute**
   - Should show consistent throughput
   - No sudden drops

3. **P95 Latency**
   - Should stay < 200ms
   - Spikes indicate problems

4. **Memory Usage**
   - Should be stable
   - Continuous growth = memory leak

5. **Error Rate**
   - Should be near 0%
   - Spikes need investigation

## What's Next?

### Run Other Scenarios

```bash
# Test connection limits (17 min)
task load:run:connection

# Test stock bot (6.5 min)
task load:run:stock

# Test resilience (10 min)
task load:run:spike

# Test stability - quick version (15 min)
task load:run:soak-short

# Test multi-room (15 min)
task load:run:multiroom
```

### Run All Tests

```bash
# Takes ~1 hour
task load:run:all
```

### Customize Tests

Edit scenario files in `tests/load/scenarios/`:
- Adjust user counts in `options.stages`
- Modify test duration
- Change thresholds
- Add custom logic

Example:
```javascript
// scenarios/02-active-chat-load.js
export const options = {
  stages: [
    { duration: '2m', target: 100 },   // Reduce from 500 to 100
    { duration: '5m', target: 100 },   // Reduce from 10m to 5m
    { duration: '1m', target: 0 },
  ],
  // ... rest of config
};
```

## Troubleshooting

### Services Won't Start

```bash
# Check what's running
docker ps

# Restart everything
docker-compose -f containers/docker-compose.yml down
task load:setup
```

### Test Fails Immediately

```bash
# Check chat-server logs
docker logs jobsity-chat-server

# Check if migrations ran
docker logs jobsity-postgres
```

### High Error Rate

Possible causes:
1. **System overloaded** - Reduce user count
2. **Rate limiting** - Expected, check limits
3. **Services crashed** - Check docker ps

```bash
# Check system resources
docker stats
```

### Grafana Dashboard Not Showing Data

```bash
# Check Prometheus is scraping
curl http://localhost:9090/api/v1/targets

# Check k6 is sending metrics
docker logs jobsity-k6
```

## Common Issues

| Issue | Solution |
|-------|----------|
| "Cannot connect to chat-server" | Run `task load:setup` |
| "Dashboard not found" | Run `./setup-grafana-k6-dashboard.sh` |
| "Too many open files" | Increase ulimit: `ulimit -n 65536` |
| High latency | Normal for first run (cold start) |
| Test takes forever | Reduce user count in scenario file |

## Tips

1. **Start small** - Reduce user counts for faster feedback
2. **Monitor live** - Keep Grafana open during tests
3. **Cool down** - Wait 30s between tests
4. **Baseline first** - Run connection test before others
5. **Document** - Screenshot Grafana for reference

## Quick Reference

```bash
# Setup
task load:setup

# Run tests
task load:run:chat          # Most important
task load:run:spike         # Test resilience
task load:run:soak-short    # Quick stability check

# View results
open http://localhost:3100  # Grafana
open http://localhost:9090  # Prometheus

# Cleanup
task load:clean
```

## Next Steps

1. ✅ Run `task load:run:chat` successfully
2. ✅ Verify results in Grafana
3. ✅ Run spike test for resilience
4. ✅ Run soak test overnight
5. ✅ Document your capacity limits
6. ✅ Set up alerts based on findings

## Need More Help?

- **Full Documentation**: `tests/load/README.md`
- **k6 Docs**: https://k6.io/docs/
- **Scenarios**: Check `tests/load/scenarios/` for examples

---

**Ready to test?**

```bash
task load:setup && task load:run:chat
```

Then open http://localhost:3100 and watch the magic! ✨
