#!/usr/bin/env python3
"""
OpenCode SSE Event Exhaustive Verification Script

Captures real SSE events from a live `opencode serve` instance and produces
a structured coverage report for all OCPart types and SSE event types.

Usage:
    python3 scripts/verify/verify_opencode_sse_events.py [--url URL] [--password PWD]
    python3 scripts/verify/verify_opencode_sse_events.py --verify --baseline=BASE.json --output=OUT.json

Examples:
    # Basic capture (auto-exit after events)
    python3 scripts/verify/verify_opencode_sse_events.py

    # Capture + save structured report
    python3 scripts/verify/verify_opencode_sse_events.py --output=report.json

    # Diff against baseline
    python3 scripts/verify/verify_opencode_sse_events.py --verify --baseline=report.json --output=new.json
"""

import argparse
import json
import sys
import time
from dataclasses import dataclass, field, asdict
from typing import Optional
from datetime import datetime

try:
    import requests
    from requests.auth import HTTPBasicAuth
except ImportError:
    print("❌ Error: requests library not installed")
    print("   Install: pip install requests")
    sys.exit(1)


@dataclass
class PartInfo:
    """Represents a single OCPart from a message.part.updated event."""
    id: str = ""
    type: str = ""
    text: str = ""
    name: str = ""
    status: str = ""
    error: str = ""
    output: str = ""
    step_number: int = 0
    total_steps: int = 0
    reason: str = ""
    has_delta: bool = False
    delta_text: str = ""
    usage_input_tokens: int = 0
    usage_output_tokens: int = 0
    cache_read: int = 0
    cache_write: int = 0


@dataclass
class EventCapture:
    """A single captured SSE event."""
    index: int
    raw_line: str
    sse_type: str
    properties: dict
    parts: list = field(default_factory=list)
    timestamp_ms: int = 0


@dataclass
class TestResult:
    """Result of a single test case."""
    test_id: str
    prompt_preview: str
    passed: bool
    events_captured: int
    event_types: list = field(default_factory=list)
    part_types: list = field(default_factory=list)
    events: list = field(default_factory=list)
    duration_ms: int = 0
    error: str = ""
    delta_examples: list = field(default_factory=list)


@dataclass
class CoverageReport:
    """Complete coverage report."""
    generated_at: str = ""
    server_url: str = ""
    total_events: int = 0
    total_tests: int = 0
    tests_passed: int = 0
    test_results: list = field(default_factory=list)
    all_event_types: list = field(default_factory=list)
    all_part_types: list = field(default_factory=list)
    metadata_fields_populated: dict = field(default_factory=dict)
    delta_examples: list = field(default_factory=list)
    missing_event_types: list = field(default_factory=list)


class Colors:
    """ANSI color codes for terminal output."""
    GREEN = '\033[92m'
    RED = '\033[91m'
    YELLOW = '\033[93m'
    BLUE = '\033[94m'
    CYAN = '\033[96m'
    RESET = '\033[0m'
    BOLD = '\033[1m'


# Expected SSE event types per spec (provider/opencode_types.go)
EXPECTED_SSE_EVENTS = {
    "message.part.updated",
    "message.updated",
    "session.status",
    "session.idle",
    "session.error",
    "permission.updated",
}

# Expected OCPart types per spec
EXPECTED_PART_TYPES = {
    "text",
    "reasoning",
    "tool",
    "step-start",
    "step-finish",
}


def parse_sse_line(line: str) -> Optional[dict]:
    """Parse a single SSE data line into a dict."""
    if not line.startswith("data:"):
        return None
    json_str = line[5:].strip()
    if not json_str or json_str == "[DONE]":
        return None
    try:
        return json.loads(json_str)
    except json.JSONDecodeError:
        return None


def extract_parts(props: dict) -> list:
    """Extract OCPart info from message.part.updated properties."""
    parts = []
    part_data = props.get("part", {})
    if not part_data:
        return parts

    part_type = part_data.get("type", "")

    # Tool parts: status is in state.status, name is in 'tool' field
    if part_type == "tool":
        state = part_data.get("state", {}) or {}
        p = PartInfo(
            id=part_data.get("id", ""),
            type=part_type,
            name=part_data.get("tool", ""),
            status=state.get("status", ""),
            error=part_data.get("error", ""),
            output=part_data.get("output", ""),
        )
    else:
        p = PartInfo(
            id=part_data.get("id", ""),
            type=part_type,
            text=part_data.get("text", ""),
            name=part_data.get("name", ""),
            status=part_data.get("status", ""),
            error=part_data.get("error", ""),
            output=part_data.get("output", ""),
            step_number=part_data.get("step_number", 0),
            total_steps=part_data.get("total_steps", 0),
            reason=part_data.get("reason", ""),
        )

    # Extract usage/tokens from part (step-finish has tokens here)
    tokens = part_data.get("tokens", {}) or {}
    if tokens:
        p.usage_input_tokens = int(tokens.get("input", 0))
        p.usage_output_tokens = int(tokens.get("output", 0))
        cache = tokens.get("cache", {}) or {}
        p.cache_read = int(cache.get("read", 0))
        p.cache_write = int(cache.get("write", 0))

    parts.append(p)
    return parts

def capture_events(url: str, session_id: str, auth: Optional, timeout: float = 30.0) -> tuple:
    """
    Capture SSE events from a session.
    Returns (events: list, error: str, duration_ms: int)
    """
    events = []
    start = time.time()
    error_msg = ""

    try:
        resp = requests.get(
            f"{url}/event",
            auth=auth,
            stream=True,
            timeout=timeout,
        )
        if resp.status_code != 200:
            return [], f"HTTP {resp.status_code}", 0

        idx = 0
        for line in resp.iter_lines(decode_unicode=True):
            if not line:
                continue

            elapsed_ms = int((time.time() - start) * 1000)

            parsed = parse_sse_line(line)
            if parsed is None:
                continue

            sse_type = parsed.get("type", "")
            raw_props = parsed.get("properties", {}) or {}

            # Stop on session.idle (natural end of turn)
            if sse_type == "session.idle":
                idx += 1
                evt = EventCapture(
                    index=idx,
                    raw_line=line[:200],
                    sse_type=sse_type,
                    properties=raw_props,
                    parts=[],
                    timestamp_ms=elapsed_ms,
                )
                events.append(evt)
                break

            # Extract parts from properties
            parts = extract_parts(raw_props)
            if not parts:
                # Fallback: try delta extraction, but use full part_data when available
                part_data = raw_props.get("part", {})
                delta = raw_props.get("delta", "")
                if delta or part_data:
                    part_type = part_data.get("type", "") if part_data else ""
                    if part_type == "tool":
                        state = part_data.get("state", {}) or {}
                        delta_part = PartInfo(
                            id=part_data.get("id", ""),
                            type=part_type,
                            has_delta=bool(delta),
                            delta_text=delta[:100] if delta else "",
                            name=part_data.get("tool", ""),
                            status=state.get("status", ""),
                            error=part_data.get("error", ""),
                            output=part_data.get("output", ""),
                        )
                    else:
                        delta_part = PartInfo(
                            id=part_data.get("id", ""),
                            type=part_type,
                            has_delta=bool(delta),
                            delta_text=delta[:100] if delta else "",
                            step_number=part_data.get("step_number", 0),
                            total_steps=part_data.get("total_steps", 0),
                            reason=part_data.get("reason", ""),
                            name=part_data.get("name", ""),
                            status=part_data.get("status", ""),
                        )
                    # Extract tokens from part (step-finish parts)
                    tokens = part_data.get("tokens", {}) or {}
                    if tokens:
                        delta_part.usage_input_tokens = int(tokens.get("input", 0))
                        delta_part.usage_output_tokens = int(tokens.get("output", 0))
                        cache = tokens.get("cache", {}) or {}
                        delta_part.cache_read = int(cache.get("read", 0))
                        delta_part.cache_write = int(cache.get("write", 0))
                    parts = [delta_part]

            # For message.updated events, extract token usage from info.tokens
            if sse_type == "message.updated" and not parts:
                info = raw_props.get("info", {}) or {}
                tokens = info.get("tokens", {}) or {}
                if tokens:
                    cache = tokens.get("cache", {}) or {}
                    token_part = PartInfo(
                        type="tokens",
                        usage_input_tokens=int(tokens.get("input", 0)),
                        usage_output_tokens=int(tokens.get("output", 0)),
                        cache_read=int(cache.get("read", 0)),
                        cache_write=int(cache.get("write", 0)),
                    )
                    parts = [token_part]

            evt = EventCapture(
                index=idx,
                raw_line=line[:200],
                sse_type=sse_type,
                properties=raw_props,
                parts=parts,
                timestamp_ms=elapsed_ms,
            )
            events.append(evt)
            idx += 1

            # Stop after timeout (safety net)
            if elapsed_ms > timeout * 1000:
                break

    except requests.exceptions.Timeout:
        error_msg = f"Timeout after {timeout}s"
    except requests.exceptions.ConnectionError as e:
        error_msg = f"Connection refused: {e}"
    except Exception as e:
        error_msg = f"Error: {e}"

    duration_ms = int((time.time() - start) * 1000)
    return events, error_msg, duration_ms


def run_test_case(
    url: str,
    auth: Optional,
    test_id: str,
    prompt: str,
    provider: str = "anthropic",
    model: str = "claude-sonnet-4-20250514",
    timeout: float = 30.0,
) -> TestResult:
    """Run a single test case: create session, send message, capture events."""
    prompt_preview = prompt[:60] + ("..." if len(prompt) > 60 else "")

    # Step 1: Create session
    try:
        resp = requests.post(
            f"{url}/session",
            json={"provider": provider, "model": model},
            auth=auth,
            timeout=10,
        )
        if resp.status_code not in (200, 201):
            return TestResult(
                test_id=test_id,
                prompt_preview=prompt_preview,
                passed=False,
                events_captured=0,
                error=f"Session creation failed: HTTP {resp.status_code}: {resp.text[:100]}",
            )
        data = resp.json()
        session_id = data.get("id") or data.get("sessionId") or data.get("session_id")
        if not session_id:
            return TestResult(
                test_id=test_id,
                prompt_preview=prompt_preview,
                passed=False,
                events_captured=0,
                error="No session_id in response",
            )
    except Exception as e:
        return TestResult(
            test_id=test_id,
            prompt_preview=prompt_preview,
            passed=False,
            events_captured=0,
            error=f"Session creation error: {e}",
        )

    # Step 2: Send message (non-blocking: fire and don't wait)
    try:
        requests.post(
            f"{url}/session/{session_id}/message",
            json={"parts": [{"type": "text", "text": prompt}]},
            auth=auth,
            timeout=5,
        )
    except Exception:
        pass  # Fire-and-forget; events will still flow

    # Step 3: Capture events
    events, error_msg, duration_ms = capture_events(url, session_id, auth, timeout)

    # Step 4: Cleanup
    try:
        requests.delete(f"{url}/session/{session_id}", auth=auth, timeout=5)
    except Exception:
        pass

    # Analyze events
    event_types = list({e.sse_type for e in events})
    part_types = set()
    for e in events:
        for p in e.parts:
            if p.type:
                part_types.add(p.type)

    # Check delta presence
    delta_examples = []
    for e in events:
        for p in e.parts:
            if p.has_delta:
                delta_examples.append({
                    "event_type": e.sse_type,
                    "part_type": p.type,
                    "delta_preview": p.delta_text[:80],
                })

    passed = len(events) > 0 and error_msg == ""

    return TestResult(
        test_id=test_id,
        prompt_preview=prompt_preview,
        passed=passed,
        events_captured=len(events),
        event_types=event_types,
        part_types=list(part_types),
        events=[
            {
                "index": e.index,
                "sse_type": e.sse_type,
                "part_types": [p.type for p in e.parts],
                "timestamp_ms": e.timestamp_ms,
                "step_number": max((p.step_number for p in e.parts), default=0),
                "total_steps": max((p.total_steps for p in e.parts), default=0),
                "status": e.parts[0].status if e.parts else "",
                "reason": e.parts[0].reason if e.parts else "",
                "name": e.parts[0].name if e.parts else "",
                "has_usage": any(p.usage_input_tokens > 0 or p.usage_output_tokens > 0 for p in e.parts),
                "has_cache": any(p.cache_read > 0 or p.cache_write > 0 for p in e.parts),
            }
            for e in events
        ],
        duration_ms=duration_ms,
        error=error_msg,
        delta_examples=delta_examples,
    )


def build_report(
    results: list,
    server_url: str,
    baseline_path: Optional[str] = None,
) -> CoverageReport:
    """Build a structured coverage report from test results."""
    report = CoverageReport(
        generated_at=datetime.now().isoformat(),
        server_url=server_url,
        total_tests=len(results),
        tests_passed=sum(1 for r in results if r.passed),
        test_results=[asdict(r) for r in results],
    )

    all_event_types = set()
    all_part_types = set()
    total_events = 0
    has_delta_count = 0

    for r in results:
        total_events += r.events_captured
        all_event_types.update(r.event_types)
        all_part_types.update(r.part_types)
        for e in r.events:
            pass  # already counted

    # Count deltas
    for r in results:
        for e in r.events:
            pass  # simplified

    # Build metadata field coverage
    metadata_fields = {
        "step_number": 0,
        "total_steps": 0,
        "reason": 0,
        "usage.input_tokens": 0,
        "usage.output_tokens": 0,
        "cache.read": 0,
        "cache.write": 0,
        "status (tool)": 0,
        "error (tool)": 0,
        "name (tool)": 0,
    }
    for r in results:
        for e in r.events:
            if e.get("step_number", 0) > 0:
                metadata_fields["step_number"] += 1
            if e.get("total_steps", 0) > 0:
                metadata_fields["total_steps"] += 1
            if e.get("reason"):
                metadata_fields["reason"] += 1
            if e.get("status"):
                metadata_fields["status (tool)"] += 1
            if e.get("name"):
                metadata_fields["name (tool)"] += 1
            if e.get("has_usage"):
                metadata_fields["usage.input_tokens"] += 1
                metadata_fields["usage.output_tokens"] += 1
            if e.get("has_cache"):
                metadata_fields["cache.read"] += 1
                metadata_fields["cache.write"] += 1

    report.all_event_types = sorted(all_event_types)
    report.all_part_types = sorted(all_part_types)
    report.total_events = total_events
    report.metadata_fields_populated = metadata_fields

    # Check missing expected types
    report.missing_event_types = sorted(EXPECTED_SSE_EVENTS - all_event_types)
    missing_parts = EXPECTED_PART_TYPES - all_part_types
    if missing_parts:
        report.missing_event_types.append(f"(parts) {sorted(missing_parts)}")

    # Aggregate delta examples from all test results
    all_deltas = []
    for r in results:
        all_deltas.extend(r.delta_examples)
    report.delta_examples = all_deltas[:20]  # Limit to 20 examples

    return report


def print_report(report: CoverageReport):
    """Print a human-readable coverage report."""
    print(f"\n{Colors.CYAN}{Colors.BOLD}{'═' * 70}{Colors.RESET}")
    print(f"{Colors.CYAN}{Colors.BOLD}  SSE Event Coverage Report{Colors.RESET}")
    print(f"{Colors.CYAN}{Colors.BOLD}  Generated: {report.generated_at}{Colors.RESET}")
    print(f"{Colors.CYAN}{Colors.BOLD}{'═' * 70}{Colors.RESET}\n")

    print(f"{Colors.BOLD}Server:{Colors.RESET} {report.server_url}")
    print(f"{Colors.BOLD}Total tests:{Colors.RESET} {report.tests_passed}/{report.total_tests} passed")
    print(f"{Colors.BOLD}Total events captured:{Colors.RESET} {report.total_events}\n")

    print(f"{Colors.BOLD}SSE Event Types Seen:{Colors.RESET}")
    if report.all_event_types:
        for t in report.all_event_types:
            marker = "✅" if t in EXPECTED_SSE_EVENTS else "⚠️"
            print(f"  {marker} {t}")
    else:
        print(f"  {Colors.RED}None captured{Colors.RESET}")

    missing_ev = [m for m in report.missing_event_types if not m.startswith("(parts)")]
    if missing_ev:
        print(f"  {Colors.YELLOW}Missing expected:{Colors.RESET} {', '.join(missing_ev)}")

    print(f"\n{Colors.BOLD}OCPart Types Seen:{Colors.RESET}")
    if report.all_part_types:
        for t in report.all_part_types:
            marker = "✅" if t in EXPECTED_PART_TYPES else "⚠️"
            print(f"  {marker} {t}")
    else:
        print(f"  {Colors.RED}None captured{Colors.RESET}")

    missing_parts = [m for m in report.missing_event_types if m.startswith("(parts)")]
    if missing_parts:
        print(f"  {Colors.YELLOW}Missing expected:{Colors.RESET} {missing_parts}")

    print(f"\n{Colors.BOLD}Metadata Field Coverage:{Colors.RESET}")
    for field_name, count in report.metadata_fields_populated.items():
        bar = "█" * min(count, 20)
        print(f"  {field_name:30s} {bar} ({count})")

    print(f"\n{Colors.BOLD}Test Case Results:{Colors.RESET}")
    for r in report.test_results:
        status_icon = f"{Colors.GREEN}✅ PASS{Colors.RESET}" if r["passed"] else f"{Colors.RED}❌ FAIL{Colors.RESET}"
        print(f"\n  {status_icon} [{r['test_id']}] {r['prompt_preview']}")
        print(f"       Events: {r['events_captured']} | Duration: {r['duration_ms']}ms")
        if r['event_types']:
            print(f"       SSE types: {', '.join(r['event_types'])}")
        if r['part_types']:
            print(f"       Part types: {', '.join(r['part_types'])}")
        if r['error']:
            print(f"       {Colors.RED}Error: {r['error']}{Colors.RESET}")

    print(f"\n{Colors.CYAN}{Colors.BOLD}{'═' * 70}{Colors.RESET}\n")


# =============================================================================
# TEST CASES
# =============================================================================

TEST_CASES = [
    {
        "id": "T1",
        "prompt": "Hello! Please respond with just 'OK' to confirm you're working.",
        "description": "Simple text response",
        "expected_events": ["message.part.updated", "message.updated"],
        "expected_parts": ["text"],
    },
    {
        "id": "T2",
        "prompt": "Explain why the sky is blue in one short paragraph.",
        "description": "Reasoning + text",
        "expected_events": ["message.part.updated"],
        "expected_parts": ["reasoning", "text"],
    },
    {
        "id": "T3",
        "prompt": "List the files in the /tmp directory using the Bash tool.",
        "description": "Tool call (Bash)",
        "expected_events": ["message.part.updated"],
        "expected_parts": ["tool"],
    },
    {
        "id": "T4",
        "prompt": "Write a brief analysis of the Go files in this directory: what does each file do?",
        "description": "Multi-step analysis (may trigger step-start/step-finish)",
        "expected_events": ["message.part.updated"],
        "expected_parts": ["step-start", "step-finish", "reasoning", "tool", "text"],
    },
    {
        "id": "T5",
        "prompt": "Run this invalid command: blahblah_invalid_command_xyz",
        "description": "Error trigger",
        "expected_events": ["session.error", "message.part.updated"],
        "expected_parts": ["tool"],
    },
    {
        "id": "T6",
        "prompt": "Show me the current date and time using the Bash tool.",
        "description": "Tool execution (safe command)",
        "expected_events": ["message.part.updated"],
        "expected_parts": ["tool"],
    },
    {
        "id": "T7",
        "prompt": "What is 2+2? Just answer the number.",
        "description": "Quick text response (for idle testing)",
        "expected_events": ["message.part.updated", "session.idle"],
        "expected_parts": ["text"],
    },
]


def main():
    parser = argparse.ArgumentParser(
        description="OpenCode SSE Event Exhaustive Verification",
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument(
        "--url",
        default="http://127.0.0.1:4096",
        help="OpenCode Server URL (default: http://127.0.0.1:4096)"
    )
    parser.add_argument(
        "--password",
        help="Password for Basic Auth (optional)"
    )
    parser.add_argument(
        "--output", "-o",
        help="Save report as JSON to this file"
    )
    parser.add_argument(
        "--verify",
        action="store_true",
        help="Verify mode: compare against baseline"
    )
    parser.add_argument(
        "--baseline", "-b",
        help="Baseline report JSON for comparison (with --verify)"
    )
    parser.add_argument(
        "--timeout", "-t",
        type=float,
        default=30.0,
        help="SSE capture timeout per test case (default: 30s)"
    )
    parser.add_argument(
        "--test", "-T",
        help="Run only a specific test ID (e.g., T1)"
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Print test cases without running"
    )

    args = parser.parse_args()

    auth = HTTPBasicAuth('', args.password) if args.password else None

    print(f"{Colors.BOLD}{Colors.BLUE}╭─ 🔍 OpenCode SSE Exhaustive Verification ────────────────{Colors.RESET}")
    print(f"{Colors.BOLD}{Colors.BLUE}│{Colors.RESET}")
    print(f"{Colors.BOLD}{Colors.BLUE}│  Server: {args.url}{Colors.RESET}")
    print(f"{Colors.BOLD}{Colors.BLUE}│  Auth:   {'Enabled' if args.password else 'Disabled'}{Colors.RESET}")
    print(f"{Colors.BOLD}{Colors.BLUE}│  Tests:  {len(TEST_CASES)}{Colors.RESET}")
    print(f"{Colors.BOLD}{Colors.BLUE}╰─────────────────────────────────────────────────────────{Colors.RESET}")

    if args.dry_run:
        print(f"\n{Colors.BOLD}Test cases (--dry-run):{Colors.RESET}")
        for tc in TEST_CASES:
            print(f"  [{tc['id']}] {tc['description']}")
            print(f"       Prompt: {tc['prompt'][:60]}...")
            print(f"       Expected parts: {', '.join(tc['expected_parts'])}")
        return

    # Check server availability
    try:
        resp = requests.get(args.url + "/global/health", auth=auth, timeout=5)
        server_ok = resp.status_code == 200
    except Exception:
        server_ok = False

    if not server_ok:
        print(f"\n{Colors.RED}❌ Error: OpenCode Server not reachable at {args.url}{Colors.RESET}")
        print(f"   Please ensure `opencode serve` is running.")
        print(f"   Start: opencode serve --port 4096")
        sys.exit(1)

    print(f"{Colors.GREEN}✅ Server is reachable{Colors.RESET}\n")

    # Run test cases
    test_cases_to_run = [tc for tc in TEST_CASES if args.test is None or tc["id"] == args.test]
    results = []

    for tc in test_cases_to_run:
        print(f"{Colors.YELLOW}Running [{tc['id']}] {tc['description']}...{Colors.RESET}", end=" ", flush=True)
        result = run_test_case(
            url=args.url,
            auth=auth,
            test_id=tc["id"],
            prompt=tc["prompt"],
            timeout=args.timeout,
        )
        results.append(result)
        status = f"{Colors.GREEN}✅{Colors.RESET}" if result.passed else f"{Colors.RED}❌{Colors.RESET}"
        print(f"{status} ({result.events_captured} events, {result.duration_ms}ms)")

    # Build report
    report = build_report(results, args.url)

    # Print report
    print_report(report)

    # Save to file
    if args.output:
        with open(args.output, "w", encoding="utf-8") as f:
            json.dump(asdict(report), f, indent=2, ensure_ascii=False)
        print(f"{Colors.GREEN}✅ Report saved to {args.output}{Colors.RESET}\n")

    # Verify mode
    if args.verify and args.baseline:
        try:
            with open(args.baseline, "r", encoding="utf-8") as f:
                baseline = json.load(f)
            print(f"\n{Colors.BOLD}Baseline vs Current Comparison:{Colors.RESET}")
            base_events = set(baseline.get("all_event_types", []))
            curr_events = set(report.all_event_types)
            new_events = curr_events - base_events
            if new_events:
                print(f"  {Colors.GREEN}New event types captured:{Colors.RESET} {', '.join(sorted(new_events))}")
            base_parts = set(baseline.get("all_part_types", []))
            curr_parts = set(report.all_part_types)
            new_parts = curr_parts - base_parts
            if new_parts:
                print(f"  {Colors.GREEN}New part types captured:{Colors.RESET} {', '.join(sorted(new_parts))}")
            if not new_events and not new_parts:
                print(f"  {Colors.YELLOW}No new types captured vs baseline{Colors.RESET}")
        except Exception as e:
            print(f"{Colors.YELLOW}⚠️ Could not compare with baseline: {e}{Colors.RESET}")

    # Exit code
    all_passed = all(r.passed for r in results)
    if not all_passed:
        print(f"{Colors.RED}❌ Some test cases failed{Colors.RESET}")
        sys.exit(1)
    else:
        print(f"{Colors.GREEN}✅ All test cases passed{Colors.RESET}")
        sys.exit(0)


if __name__ == "__main__":
    main()
