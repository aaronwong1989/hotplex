#!/usr/bin/env python3
"""
Claude Code stream-json Token Data Verification Script

This script verifies that Claude Code CLI returns token usage data in stream-json output mode.

Usage:
    python3 scripts/verify_claude_stream_tokens.py

Requirements:
    - Claude Code CLI installed and configured
    - ANTHROPIC_AUTH_TOKEN environment variable set
"""

import json
import subprocess
import sys
from typing import Dict, Any, Optional


def run_claude_stream_json(prompt: str) -> list[Dict[str, Any]]:
    """
    Run Claude Code with stream-json output format and return parsed events.

    Args:
        prompt: The prompt to send to Claude

    Returns:
        List of parsed JSON events
    """
    cmd = [
        "claude",
        "-p", prompt,
        "--verbose",
        "--output-format", "stream-json"
    ]

    print(f"🚀 Running: {' '.join(cmd)}")
    print(f"📝 Prompt: {prompt}")
    print("-" * 80)

    events = []
    try:
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            timeout=60
        )

        if result.returncode != 0:
            print(f"❌ Command failed with return code {result.returncode}")
            print(f"stderr: {result.stderr}")
            return events

        # Parse each line as a JSON event
        for line in result.stdout.strip().split('\n'):
            if line.strip():
                try:
                    event = json.loads(line)
                    events.append(event)
                except json.JSONDecodeError as e:
                    print(f"⚠️  Failed to parse line: {e}")
                    print(f"   Line: {line[:100]}...")

    except subprocess.TimeoutExpired:
        print("❌ Command timed out after 60 seconds")
    except Exception as e:
        print(f"❌ Error running command: {e}")

    return events


def extract_token_data(events: list[Dict[str, Any]]) -> Optional[Dict[str, Any]]:
    """
    Extract token usage data from stream-json events.

    Token data can appear in:
    - message.stop event (usage field)
    - result event (usage field)

    Args:
        events: List of parsed JSON events

    Returns:
        Dictionary containing token data if found, None otherwise
    """
    token_data = None

    for i, event in enumerate(events):
        event_type = event.get("type", "")

        # Check for message.stop events
        if event_type == "message.stop":
            usage = event.get("message", {}).get("usage", {})
            if usage:
                token_data = {
                    "event_index": i,
                    "event_type": event_type,
                    "input_tokens": usage.get("input_tokens", 0),
                    "output_tokens": usage.get("output_tokens", 0),
                    "cache_creation_input_tokens": usage.get("cache_creation_input_tokens", 0),
                    "cache_read_input_tokens": usage.get("cache_read_input_tokens", 0),
                }
                print(f"✅ Found token data in message.stop event (index {i})")
                break

        # Check for result events
        elif event_type == "result":
            usage = event.get("usage", {})
            if usage:
                token_data = {
                    "event_index": i,
                    "event_type": event_type,
                    "input_tokens": usage.get("input_tokens", 0),
                    "output_tokens": usage.get("output_tokens", 0),
                    "cache_creation_input_tokens": usage.get("cache_creation_input_tokens", 0),
                    "cache_read_input_tokens": usage.get("cache_read_input_tokens", 0),
                }
                print(f"✅ Found token data in result event (index {i})")
                break

    return token_data


def print_event_summary(events: list[Dict[str, Any]]) -> None:
    """Print a summary of all events found in the stream."""
    print("\n📊 Event Summary:")
    print("-" * 80)
    print(f"Total events: {len(events)}")

    event_types = {}
    for event in events:
        event_type = event.get("type", "unknown")
        event_types[event_type] = event_types.get(event_type, 0) + 1

    print("\nEvent type distribution:")
    for event_type, count in sorted(event_types.items()):
        print(f"  • {event_type}: {count}")


def main():
    """Main entry point."""
    print("=" * 80)
    print("Claude Code stream-json Token Data Verification")
    print("=" * 80)

    # Simple test prompt
    prompt = "What is 2+2? Just answer with the number."

    # Run Claude Code with stream-json output
    events = run_claude_stream_json(prompt)

    if not events:
        print("\n❌ No events captured")
        sys.exit(1)

    # Print event summary
    print_event_summary(events)

    # Extract token data
    print("\n" + "=" * 80)
    print("Token Data Extraction")
    print("=" * 80)

    token_data = extract_token_data(events)

    if token_data:
        print("\n✅ Token data found:")
        print("-" * 80)
        print(f"Event type: {token_data['event_type']}")
        print(f"Event index: {token_data['event_index']}")
        print("\nToken counts:")
        print(f"  • Input tokens: {token_data['input_tokens']}")
        print(f"  • Output tokens: {token_data['output_tokens']}")
        print(f"  • Cache creation input tokens: {token_data['cache_creation_input_tokens']}")
        print(f"  • Cache read input tokens: {token_data['cache_read_input_tokens']}")

        # Check for cache token fields
        if token_data['cache_creation_input_tokens'] > 0 or token_data['cache_read_input_tokens'] > 0:
            print("\n✅ Cache token fields ARE present in stream-json output")
        else:
            print("\n⚠️  Cache token fields are ZERO (may not be using caching)")

        print("\n" + "=" * 80)
        print("✅ VERIFICATION PASSED: Claude Code returns token data in stream-json mode")
        print("=" * 80)
    else:
        print("\n❌ No token data found in any event")
        print("\n" + "=" * 80)
        print("❌ VERIFICATION FAILED: Token data not present in stream-json output")
        print("=" * 80)
        sys.exit(1)


if __name__ == "__main__":
    main()
