# Security Incident Response Guide

This document outlines the procedures for responding to security incidents in the Hotplex system.

## Table of Contents

1. [Incident Classification](#incident-classification)
2. [Response Workflow](#response-workflow)
3. [Incident Categories](#incident-categories)
4. [Recovery Procedures](#recovery-procedures)
5. [Post-Incident](#post-incident)

---

## Incident Classification

### Severity Levels

| Level | Name | Description | Response Time |
|-------|------|-------------|---------------|
| P1 | Emergency | Active breach, data exfiltration, system compromise | Immediate |
| P2 | Critical | Successful attack, privilege escalation, malware detection | 1 hour |
| P3 | High | Blocked attack attempts, suspicious behavior | 4 hours |
| P4 | Medium | Policy violations, minor security events | 24 hours |
| P5 | Low | Informational security events | 48 hours |

### Incident Types

- **Threat Detection (AI Guard)**: Malicious inputs detected by AI Guard
- **Danger Block**: Dangerous shell commands blocked by detector
- **Bypass Attempt**: Unauthorized attempt to bypass security controls
- **Workspace Violation**: Unauthorized workspace access attempt
- **Landlock Violation**: Filesystem sandbox restriction triggered
- **Permission Denial**: Access control violations
- **Anomaly Detection**: Unusual behavior patterns detected

---

## Response Workflow

### Phase 1: Detection & Triage (0-15 minutes)

1. **Receive Alert**
   - Check alert channel (Slack, webhook, logs)
   - Note alert severity and category
   - Identify affected systems/users

2. **Initial Assessment**
   ```
   Questions to answer:
   - What triggered the alert?
   - Is this a true positive or false positive?
   - What systems/users are affected?
   - Is the attack ongoing or completed?
   ```

3. **Severity Classification**
   - Assign severity level (P1-P5)
   - Determine if escalation is required

### Phase 2: Containment (15-60 minutes)

1. **Immediate Actions**
   - Isolate affected sessions
   - Disable compromised accounts
   - Block attacker's IP if identified
   - Preserve evidence

2. **Short-term Containment**
   - Review recent audit logs
   - Identify scope of compromise
   - Implement additional monitoring

### Phase 3: Investigation (1-24 hours)

1. **Gather Evidence**
   - Export audit logs: `journalctl -u hotplexd -n 1000`
   - Export telemetry: Check OTLP traces
   - Export security metrics: Use `/metrics` endpoint

2. **Analyze Attack Vector**
   - Review blocked commands
   - Review AI Guard verdicts
   - Review workspace access logs

3. **Identify Root Cause**
   - How did the attack attempt occur?
   - Was it user-initiated or automated?
   - Are there other vulnerable inputs?

### Phase 4: Recovery (24-72 hours)

1. **System Recovery**
   - Restore any modified configurations
   - Verify system integrity
   - Resume normal operations

2. **User Recovery**
   - Restore user access if revoked
   - Communicate with affected users

---

## Incident Categories

### 1. Threat Detection (AI Guard)

**Symptoms:**
- `Alert: Threat Detected` with category `threat_detection`
- High threat score (>0.8)

**Response:**
1. Check AI Guard logs for input details
2. Review user's session history
3. Determine if input was malicious or false positive
4. If malicious: revoke session, notify user

**Recovery:**
- No system changes required
- Update AI Guard rules if needed
- Document incident in logs

### 2. Danger Block

**Symptoms:**
- `Alert: Dangerous Command Blocked`
- Command matched security rules

**Response:**
1. Review the blocked command
2. Check if legitimate operation
3. If malicious: user education needed
4. If false positive: update rules

**Recovery:**
- Explain blocked command to user
- Provide safe alternatives
- Consider rule refinement

### 3. Bypass Attempt

**Symptoms:**
- `Alert: Security Bypass Attempt`
- `bypass_success: true` is critical

**Response:**
P1 if bypass successful:
1. Immediately revoke session
2. Audit all user sessions
3. Review security logs
4. Consider IP blocking

**Recovery:**
- Force session re-authentication
- Review authorization rules
- Enable enhanced monitoring

### 4. Workspace Violation

**Symptoms:**
- `Alert: Workspace Access Denied`
- Unauthorized path access attempt

**Response:**
1. Verify workspace isolation
2. Check if legitimate cross-workspace access needed
3. Review Landlock configuration

**Recovery:**
- Adjust workspace permissions if valid
- No system recovery needed for blocked attempts

### 5. Landlock Violation

**Symptoms:**
- `Alert: Landlock Violation`
- Filesystem access denied

**Response:**
1. Review the denied operation
2. Verify file path restrictions
3. Determine if legitimate access needed

**Recovery:**
- Add path to allowed list if valid
- No system recovery needed

---

## Recovery Procedures

### Quick Recovery Checklist

- [ ] Verify alert is resolved
- [ ] Confirm no ongoing attack
- [ ] Check system services running
- [ ] Verify audit logging working
- [ ] Clear alert from active queue

### Full System Recovery

If system compromise is confirmed:

1. **Isolate System**
   ```bash
   # Disable network access
   systemctl stop hotplexd
   
   # Review running processes
   ps aux | grep -v grep
   ```

2. **Preserve Evidence**
   ```bash
   # Export all logs
   tar -czf incident-$(date +%Y%m%d).tar.gz \
     /var/log/hotplex/ \
     ~/.hotplex/audit/
   ```

3. **Restore from Backup**
   - Identify last known good state
   - Restore configuration files
   - Verify integrity checksums

4. **Verify Integrity**
   ```bash
   # Check system health
   curl http://localhost:8080/health
   
   # Check security metrics
   curl http://localhost:8080/metrics | grep security
   ```

5. **Resume Operations**
   ```bash
   systemctl start hotplexd
   ```

### Account Recovery

If user account compromised:

1. **Disable Account**
   ```bash
   # Revoke API keys
   # Disable OAuth tokens
   ```

2. **Verify Actions**
   - Review all actions taken by account
   - Revert any unauthorized changes

3. **Restore Access**
   - Force password reset
   - Re-enable with new credentials
   - Enable enhanced monitoring

---

## Post-Incident

### Documentation Requirements

For each incident, document:
- Date/time of detection
- Severity level and rationale
- Systems/users affected
- Attack vector description
- Response actions taken
- Lessons learned

### Review Meeting

Conduct review within 72 hours of incident closure:
1. What worked well?
2. What could be improved?
3. Are additional safeguards needed?
4. Update detection rules if needed

### Rule Updates

Based on incidents:
- Add new detection patterns
- Refine false positive rules
- Update severity levels
- Enhance monitoring

---

## Contact Information

| Role | Contact |
|------|---------|
| Security Lead | [To be configured] |
| On-call | [To be configured] |
| Management | [To be configured] |

---

## Appendix: Useful Commands

### View Security Alerts
```bash
# Check recent alerts
curl http://localhost:8080/api/v1/security/alerts

# Filter by severity
curl http://localhost:8080/api/v1/security/alerts?severity=critical
```

### View Audit Logs
```bash
# View recent audit events
cat ~/.hotplex/audit/*.jsonl | jq '.last(20)'

# Filter by category
cat ~/.hotplex/audit/*.jsonl | jq 'select(.category == "danger_block")'
```

### View Metrics
```bash
# Security metrics summary
curl http://localhost:8080/metrics | grep -E "security|threat|danger"

# Window rate (attack frequency)
curl http://localhost:8080/api/v1/metrics/security
```

### Enable Debug Logging
```bash
# Set environment variable
export LOG_LEVEL=debug

# Restart service
systemctl restart hotplexd
```

---

_This document is part of the Hotplex security observability suite. For questions, refer to the architecture documentation._
