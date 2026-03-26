#!/usr/bin/env python3
"""
Claude Code stream-json Context Window Verification Script

This script verifies the actual data structure returned by Claude Code CLI
in stream-json mode to understand how to correctly calculate context window usage.

Usage:
    python3 scripts/verify_context_window.py

Requirements:
    - Claude Code CLI installed and configured
"""

import json
import subprocess
import sys
from typing import Dict, Any, Optional, List


def run_claude_stream_json(prompt: str, session_id: Optional[str] = None) -> List[Dict[str, Any]]:
    """
    Run Claude Code with stream-json output format and return parsed events.

    Args:
        prompt: The prompt to send to Claude
        session_id: Optional session ID for multi-turn conversation

    Returns:
        List of parsed JSON events
    """
    cmd = [
        "claude",
        "-p", prompt,
        "--verbose",
        "--output-format", "stream-json"
    ]

    if session_id:
        cmd.extend(["--resume", session_id])

    print(f"🚀 Running: {' '.join(cmd)}")
    print(f"📝 Prompt: {prompt}")
    if session_id:
        print(f"🔑 Session ID: {session_id}")
    print("-" * 80)

    events = []
    try:
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            timeout=120
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
        print("❌ Command timed out after 120 seconds")
    except Exception as e:
        print(f"❌ Error running command: {e}")

    return events


def extract_context_window_data(events: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
    """
    Extract all context window related data from stream-json events.

    Returns:
        List of dictionaries containing context window data
    """
    context_data = []

    for i, event in enumerate(events):
        event_type = event.get("type", "")

        # Check events that might contain usage data
        if event_type in ["message.stop", "result"]:
            data = {
                "event_index": i,
                "event_type": event_type,
            }

            # Check message.usage (for message.stop events)
            if "message" in event and "usage" in event["message"]:
                usage = event["message"]["usage"]
                data["source"] = "message.usage"
                data.update({
                    "input_tokens": usage.get("input_tokens", 0),
                    "output_tokens": usage.get("output_tokens", 0),
                    "cache_creation_input_tokens": usage.get("cache_creation_input_tokens", 0),
                    "cache_read_input_tokens": usage.get("cache_read_input_tokens", 0),
                })

                # Check for context window fields
                data["context_window_size"] = event.get("message", {}).get("context_window_size")
                data["remaining_percentage"] = event.get("message", {}).get("remaining_percentage")
                data["used_percentage"] = event.get("message", {}).get("used_percentage")

                context_data.append(data)

            # Check top-level usage (for result events)
            elif "usage" in event:
                usage = event["usage"]
                data["source"] = "usage"
                data.update({
                    "input_tokens": usage.get("input_tokens", 0),
                    "output_tokens": usage.get("output_tokens", 0),
                    "cache_creation_input_tokens": usage.get("cache_creation_input_tokens", 0),
                    "cache_read_input_tokens": usage.get("cache_read_input_tokens", 0),
                })

                # Check for context window fields at top level
                data["context_window_size"] = event.get("context_window_size")
                data["remaining_percentage"] = event.get("remaining_percentage")
                data["used_percentage"] = event.get("used_percentage")

                context_data.append(data)

    return context_data


def print_full_event_structure(events: List[Dict[str, Any]], max_events: int = 3) -> None:
    """Print the full structure of the first few events for analysis."""
    print(f"\n🔍 Full Event Structure (first {max_events} events):")
    print("=" * 80)

    for i, event in enumerate(events[:max_events]):
        print(f"\nEvent #{i} (type: {event.get('type', 'unknown')}):")
        print("-" * 80)
        print(json.dumps(event, indent=2, sort_keys=False))
        print("-" * 80)


def analyze_context_calculation(context_data: List[Dict[str, Any]]) -> None:
    """
    Analyze and compare different context window calculation methods.

    Based on statusline.zsh reference:
    - context_window.context_window_size
    - context_window.remaining_percentage
    - context_window.used_percentage
    """
    print("\n" + "=" * 80)
    print("📊 Context Window Calculation Analysis")
    print("=" * 80)

    if not context_data:
        print("❌ No context window data found")
        return

    for data in context_data:
        print(f"\nEvent #{data['event_index']} ({data['event_type']}):")
        print(f"Source: {data['source']}")
        print("-" * 80)

        # Print token counts
        print("\nToken counts:")
        print(f"  • Input tokens: {data.get('input_tokens', 0)}")
        print(f"  • Output tokens: {data.get('output_tokens', 0)}")
        print(f"  • Cache creation input tokens: {data.get('cache_creation_input_tokens', 0)}")
        print(f"  • Cache read input tokens: {data.get('cache_read_input_tokens', 0)}")

        # Print context window fields (if present)
        print("\nContext window fields:")
        ctx_size = data.get('context_window_size')
        remaining = data.get('remaining_percentage')
        used = data.get('used_percentage')

        if ctx_size is not None:
            print(f"  • Context window size: {ctx_size}")
        else:
            print(f"  • Context window size: NOT PRESENT")

        if remaining is not None:
            print(f"  • Remaining percentage: {remaining}%")
        else:
            print(f"  • Remaining percentage: NOT PRESENT")

        if used is not None:
            print(f"  • Used percentage: {used}%")
        else:
            print(f"  • Used percentage: NOT PRESENT")

        # Calculate our own percentage (current HotPlex method)
        input_tokens = data.get('input_tokens', 0)
        cache_read = data.get('cache_read_input_tokens', 0)
        cache_write = data.get('cache_creation_input_tokens', 0)
        total_input = input_tokens + cache_read + cache_write

        CONTEXT_WINDOW = 200000
        calculated_percent = (total_input / CONTEXT_WINDOW * 100) if total_input > 0 else 0

        print("\nCalculated (HotPlex method):")
        print(f"  • Total input tokens: {total_input}")
        print(f"    = {input_tokens} (input) + {cache_read} (cache_read) + {cache_write} (cache_write)")
        print(f"  • Context window size: {CONTEXT_WINDOW}")
        print(f"  • Calculated used percentage: {calculated_percent:.3f}%")

        # Compare with actual (if present)
        if used is not None:
            print(f"  • Actual used percentage: {used}%")
            diff = abs(calculated_percent - used)
            if diff > 0.1:
                print(f"  ⚠️  DIFFERENCE: {diff:.3f}% (calculation may be wrong)")
            else:
                print(f"  ✅ MATCH: Difference < 0.1%")


def main():
    """Main entry point."""
    print("=" * 80)
    print("Claude Code stream-json Context Window Verification")
    print("=" * 80)

    # Test 1: Simple single-turn conversation
    print("\n🔹 Test 1: Single-turn conversation")
    print("=" * 80)
    prompt1 = "What is 2+2? Just answer with the number."
    events1 = run_claude_stream_json(prompt1)

    if not events1:
        print("\n❌ No events captured for test 1")
        sys.exit(1)

    # Print full structure of first few events
    print_full_event_structure(events1, max_events=5)

    # Extract and analyze context window data
    context_data1 = extract_context_window_data(events1)
    analyze_context_calculation(context_data1)

    # Print event type summary
    print("\n📊 Event Summary:")
    print("-" * 80)
    print(f"Total events: {len(events1)}")
    event_types = {}
    for event in events1:
        event_type = event.get("type", "unknown")
        event_types[event_type] = event_types.get(event_type, 0) + 1
    print("\nEvent type distribution:")
    for event_type, count in sorted(event_types.items()):
        print(f"  • {event_type}: {count}")

    # Summary
    print("\n" + "=" * 80)
    print("📋 VERIFICATION SUMMARY")
    print("=" * 80)

    if context_data1:
        has_used_pct = any(d.get('used_percentage') is not None for d in context_data1)
        has_remaining_pct = any(d.get('remaining_percentage') is not None for d in context_data1)
        has_ctx_size = any(d.get('context_window_size') is not None for d in context_data1)

        print("\nContext window fields present in stream-json output:")
        print(f"  • used_percentage: {'✅ YES' if has_used_pct else '❌ NO'}")
        print(f"  • remaining_percentage: {'✅ YES' if has_remaining_pct else '❌ NO'}")
        print(f"  • context_window_size: {'✅ YES' if has_ctx_size else '❌ NO'}")

        print("\n" + "=" * 80)
        if has_used_pct or has_remaining_pct or has_ctx_size:
            print("✅ VERIFICATION PASSED: Claude Code provides context window data")
            print("=" * 80)
        else:
            print("⚠️  VERIFICATION RESULT: Context window data not directly provided")
            print("   → Need to calculate from token counts")
            print("=" * 80)
    else:
        print("\n❌ No context window data found in any event")
        print("=" * 80)
        sys.exit(1)


if __name__ == "__main__":
    main()
